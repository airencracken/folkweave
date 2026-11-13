package folkweave

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"github.com/airencracken/comfylib/displaytime"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/airencracken/comfylib/invitation"
	"github.com/airencracken/comfylib/memberprofile"
	"github.com/airencracken/comfylib/token"
	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

var (
	ErrInvalid      = errors.New("check the submitted details")
	ErrForbidden    = errors.New("you do not have permission to do that")
	ErrInvitation   = errors.New("this invitation is unavailable")
	ErrConflict     = errors.New("this changed since you opened it; reload and try again")
	ErrUsername     = errors.New("that username is already in use")
	usernamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{2,31}$`)
)

type Store struct {
	db   *sql.DB
	Path string
}
type User struct {
	Timezone string `json:"-"`
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
	Bio      string `json:"bio"`
	Role     string `json:"role"`
	Disabled bool   `json:"-"`
}

func (u User) Owner() bool { return u.Role == "owner" && !u.Disabled }
func (u User) Initial() string {
	for _, r := range u.Name {
		return strings.ToUpper(string(r))
	}
	return "?"
}

type Settings struct{ Title, About string }
type Invitation struct {
	ID            int64
	Expires       time.Time
	MaxUses, Uses int
	Revoked       bool
}

func (i Invitation) Status() string {
	return (invitation.State{Revoked: i.Revoked, ExpiresAt: &i.Expires, MaxUses: i.MaxUses, Uses: i.Uses}).Status(time.Now()).Label()
}

func Open(dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return nil, err
	}
	directory, err := os.Lstat(dataDir)
	if err != nil {
		return nil, err
	}
	if !directory.IsDir() || directory.Mode().Perm()&0077 != 0 {
		return nil, errors.New("data directory must be private (mode 0700), not a symlink")
	}
	path, err := filepath.Abs(filepath.Join(dataDir, "folkweave.db"))
	if err != nil {
		return nil, err
	}
	if existing, err := os.Lstat(path); err == nil {
		if !existing.Mode().IsRegular() || existing.Mode().Perm()&0077 != 0 {
			return nil, errors.New("database must be a private regular file (mode 0600)")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = f.Close(); err != nil {
		return nil, err
	}
	dsn := (&url.URL{Scheme: "file", Path: path}).String() + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(FULL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	var known bool
	if err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM sqlite_schema WHERE type='table' AND name='schema_version')").Scan(&known); err != nil {
		db.Close()
		return nil, err
	}
	if known {
		var n, low, high int
		if err = db.QueryRow("SELECT count(*),coalesce(min(version),0),coalesce(max(version),0) FROM schema_version").Scan(&n, &low, &high); err != nil {
			db.Close()
			return nil, err
		}
		if n != 1 || low != 1 || high != 1 {
			db.Close()
			return nil, errors.New("unsupported Folkweave schema version")
		}
	}
	if _, err = db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	if err := displaytime.Initialize(context.Background(), db); err != nil {
		return nil, errors.Join(err, db.Close())
	}
	if err := retireActivity(context.Background(), db); err != nil {
		return nil, errors.Join(err, db.Close())
	}
	return &Store{db: db, Path: path}, nil
}
func (s *Store) Close() error { return s.db.Close() }
func (s *Store) Ready(ctx context.Context) bool {
	var n int
	return s.db.QueryRowContext(ctx, "SELECT count(*) FROM users WHERE role='owner'").Scan(&n) == nil && n > 0
}

func validText(value string, limit int, multiline bool) bool {
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > limit {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) && (!multiline || (r != '\n' && r != '\t' && r != '\r')) {
			return false
		}
	}
	return true
}
func ValidateCredentials(username, password string) error {
	if !usernamePattern.MatchString(username) {
		return errors.New("use 3–32 lowercase letters, numbers or underscores; start with a letter")
	}
	if !utf8.ValidString(password) || utf8.RuneCountInString(password) < 12 {
		return errors.New("choose a password with at least 12 characters")
	}
	if len(password) > 72 {
		return errors.New("use a shorter password")
	}
	return nil
}
func HashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(b), err
}

// Bootstrap initializes a new gathering and refuses any existing accounts.
func (s *Store) Bootstrap(ctx context.Context, username, hash, name, title string) error {
	return s.provisionOwner(ctx, username, hash, name, title, true)
}

// CreateOwner provisions a new owner without changing existing accounts or
// gathering settings. The first account and settings are created atomically.
func (s *Store) CreateOwner(ctx context.Context, username, hash, name, title string) error {
	return s.provisionOwner(ctx, username, hash, name, title, false)
}

func (s *Store) provisionOwner(ctx context.Context, username, hash, name, title string, initialOnly bool) error {
	username = strings.ToLower(strings.TrimSpace(username))
	name = strings.TrimSpace(name)
	title = strings.TrimSpace(title)
	if !usernamePattern.MatchString(username) || name == "" || !validText(name, 80, false) || title == "" || !validText(title, 100, false) {
		return ErrInvalid
	}
	if _, err := bcrypt.Cost([]byte(hash)); err != nil {
		return ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if initialOnly {
		var count int
		if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM users").Scan(&count); err != nil {
			return err
		}
		if count != 0 {
			return ErrConflict
		}
	}
	res, err := tx.ExecContext(ctx, "INSERT INTO users(username,password_hash,name,role,created_at) VALUES(?,?,?,'owner',?) ON CONFLICT(username) DO NOTHING", username, hash, name, time.Now().UnixMilli())
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrUsername
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO settings(id,title) VALUES(1,?) ON CONFLICT(id) DO NOTHING", title); err != nil {
		return err
	}
	return tx.Commit()
}

type scanner interface{ Scan(...any) error }

const userColumns = "id,username,name,bio,role,disabled"

func scanUser(row scanner) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Username, &u.Name, &u.Bio, &u.Role, &u.Disabled)
	return u, err
}
func (s *Store) User(ctx context.Context, id int64) (User, error) {
	return scanUser(s.db.QueryRowContext(ctx, "SELECT "+userColumns+" FROM users WHERE id=?", id))
}
func (s *Store) Credentials(ctx context.Context, username string) (User, string, error) {
	var id int64
	var hash string
	err := s.db.QueryRowContext(ctx, "SELECT id,password_hash FROM users WHERE username=?", strings.ToLower(strings.TrimSpace(username))).Scan(&id, &hash)
	if err != nil {
		return User{}, "", err
	}
	u, err := s.User(ctx, id)
	return u, hash, err
}
func (s *Store) Settings(ctx context.Context) (Settings, error) {
	var v Settings
	err := s.db.QueryRowContext(ctx, "SELECT title,about FROM settings WHERE id=1").Scan(&v.Title, &v.About)
	return v, err
}
func requireUser(ctx context.Context, tx *sql.Tx, id int64, owner bool) error {
	var role string
	var disabled bool
	err := tx.QueryRowContext(ctx, "SELECT role,disabled FROM users WHERE id=?", id).Scan(&role, &disabled)
	if errors.Is(err, sql.ErrNoRows) || disabled || (owner && role != "owner") {
		return ErrForbidden
	}
	return err
}
func (s *Store) NewInvitation(ctx context.Context, creator int64, uses int) (string, error) {
	if uses < 1 || uses > 50 {
		return "", ErrInvalid
	}
	secret, hash := token.New()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if err = requireUser(ctx, tx, creator, true); err != nil {
		return "", err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO invitations(token_hash,creator_id,expires_at,max_uses) VALUES(?,?,?,?)", hash, creator, time.Now().Add(7*24*time.Hour).UnixMilli(), uses)
	if err != nil {
		return "", err
	}
	return secret, tx.Commit()
}
func (s *Store) Invitations(ctx context.Context) ([]Invitation, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id,expires_at,max_uses,uses,revoked FROM invitations ORDER BY id DESC LIMIT 100")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Invitation{}
	for rows.Next() {
		var i Invitation
		var stamp int64
		if err = rows.Scan(&i.ID, &stamp, &i.MaxUses, &i.Uses, &i.Revoked); err != nil {
			return nil, err
		}
		i.Expires = time.UnixMilli(stamp)
		out = append(out, i)
	}
	return out, rows.Err()
}
func (s *Store) RevokeInvitation(ctx context.Context, actor, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = requireUser(ctx, tx, actor, true); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, "UPDATE invitations SET revoked=1 WHERE id=?", id)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return sql.ErrNoRows
	}
	return tx.Commit()
}
func (s *Store) Register(ctx context.Context, username, hash, name, secret string) (int64, error) {
	username = strings.ToLower(strings.TrimSpace(username))
	name = strings.TrimSpace(name)
	if !usernamePattern.MatchString(username) || name == "" || !validText(name, 80, false) || !validToken(secret) {
		return 0, ErrInvalid
	}
	if _, err := bcrypt.Cost([]byte(hash)); err != nil {
		return 0, ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, "UPDATE invitations SET uses=uses+1 WHERE token_hash=? AND revoked=0 AND expires_at>? AND uses<max_uses", token.Hash(secret), time.Now().UnixMilli())
	if err != nil {
		return 0, err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return 0, ErrInvitation
	}
	var exists int
	if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM users WHERE username=?", username).Scan(&exists); err != nil {
		return 0, err
	}
	if exists > 0 {
		return 0, ErrUsername
	}
	res, err = tx.ExecContext(ctx, "INSERT INTO users(username,password_hash,name,role,created_at) VALUES(?,?,?,'member',?)", username, hash, name, time.Now().UnixMilli())
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, tx.Commit()
}
func (s *Store) Session(ctx context.Context, hash string) (User, error) {
	return scanUser(s.db.QueryRowContext(ctx, "SELECT u.id,u.username,u.name,u.bio,u.role,u.disabled FROM users u JOIN sessions s ON s.user_id=u.id WHERE s.token_hash=? AND s.expires_at>? AND u.disabled=0", hash, time.Now().UnixMilli()))
}
func (s *Store) NewSession(ctx context.Context, userID int64) (string, error) {
	secret, hash := token.New()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if err = requireUser(ctx, tx, userID, false); err != nil {
		return "", err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM sessions WHERE expires_at<=?", time.Now().UnixMilli()); err != nil {
		return "", err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO sessions VALUES(?,?,?)", hash, userID, time.Now().Add(7*24*time.Hour).UnixMilli())
	if err != nil {
		return "", err
	}
	return secret, tx.Commit()
}
func (s *Store) DeleteSession(ctx context.Context, hash string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE token_hash=?", hash)
	return err
}

// SetPassword is an administrative CLI operation; every existing session is revoked.
func (s *Store) SetPassword(ctx context.Context, username, hash string) error {
	if _, err := bcrypt.Cost([]byte(hash)); err != nil {
		return ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var id int64
	if err = tx.QueryRowContext(ctx, "SELECT id FROM users WHERE username=?", strings.ToLower(strings.TrimSpace(username))).Scan(&id); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE users SET password_hash=? WHERE id=?", hash, id); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM sessions WHERE user_id=?", id); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) People(ctx context.Context, includeDisabled ...bool) ([]User, error) {
	query := "SELECT " + userColumns + " FROM users"
	if len(includeDisabled) == 0 || !includeDisabled[0] {
		query += " WHERE disabled=0"
	}
	rows, err := s.db.QueryContext(ctx, query+" ORDER BY name COLLATE NOCASE,id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []User{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}
func (s *Store) Profile(ctx context.Context, id int64, name, bio string) error {
	p, err := memberprofile.Normalize(memberprofile.Profile{Name: name, Bio: bio})
	if err != nil || p.Name == "" {
		return ErrInvalid
	}
	result, err := s.db.ExecContext(ctx, "UPDATE users SET name=?,bio=? WHERE id=? AND disabled=0", p.Name, p.Bio, id)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return ErrForbidden
	}
	return nil
}
func (s *Store) SetDisabled(ctx context.Context, actor, id int64, disabled bool) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = requireUser(ctx, tx, actor, true); err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx, "UPDATE users SET disabled=? WHERE id=? AND role!='owner'", disabled, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return ErrForbidden
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM sessions WHERE user_id=?", id); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) Configure(ctx context.Context, actor int64, title, about string) error {
	title = strings.TrimSpace(title)
	about = strings.TrimSpace(about)
	if title == "" || !validText(title, 100, false) || !validText(about, 2000, true) {
		return ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = requireUser(ctx, tx, actor, true); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "UPDATE settings SET title=?,about=? WHERE id=1", title, about)
	if err != nil {
		return err
	}
	return tx.Commit()
}
func validToken(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, r := range s {
		if !strings.ContainsRune("0123456789abcdef", r) {
			return false
		}
	}
	return true
}

// Snapshot includes photos in the same SQLite transaction domain as their posts.
func (s *Store) Snapshot(ctx context.Context, directory string, limit int64) error {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	var pages, size int64
	if err := conn.QueryRowContext(ctx, "PRAGMA page_count").Scan(&pages); err != nil {
		return err
	}
	if err := conn.QueryRowContext(ctx, "PRAGMA page_size").Scan(&size); err != nil {
		return err
	}
	if pages > limit/max(size, 1) {
		return fmt.Errorf("snapshot exceeds its byte limit")
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return err
	}
	path := filepath.Join(directory, "folkweave.db")
	if _, err := conn.ExecContext(ctx, "VACUUM INTO ?", path); err != nil {
		return err
	}
	if err := os.Chmod(path, 0600); err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Size() > limit {
		os.Remove(path)
		return errors.New("snapshot exceeds its byte limit")
	}
	db, err := sql.Open("sqlite", (&url.URL{Scheme: "file", Path: path, RawQuery: "mode=ro"}).String())
	if err != nil {
		return err
	}
	defer db.Close()
	var result string
	if err = db.QueryRowContext(ctx, "PRAGMA quick_check").Scan(&result); err != nil {
		return err
	}
	if result != "ok" {
		return errors.New("snapshot verification failed")
	}
	return nil
}

func (s *Store) Check(ctx context.Context) error {
	var result string
	if err := s.db.QueryRowContext(ctx, "PRAGMA quick_check").Scan(&result); err != nil {
		return err
	}
	if result != "ok" {
		return errors.New("database integrity check failed")
	}
	var version int
	if err := s.db.QueryRowContext(ctx, "SELECT version FROM schema_version").Scan(&version); err != nil {
		return err
	}
	if version != 1 {
		return errors.New("unsupported database schema")
	}
	return nil
}
