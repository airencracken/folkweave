package folkweave

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const PageSize = 20
const MaxPostCharacters = 10000
const MaxPhotos = 4

type Photo struct {
	ID     string `json:"id"`
	Alt    string `json:"alt"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Data   []byte `json:"-"`
}

func (p Photo) URL() string { return "/photos/" + p.ID }

func (p Photo) Srcset() string {
	var candidates []string
	for _, width := range []int{320, 640, 960} {
		if p.Width <= width {
			candidates = append(candidates, fmt.Sprintf("%s %dw", p.URL(), p.Width))
			break
		}
		candidates = append(candidates, fmt.Sprintf("%s?width=%d %dw", p.URL(), width, width))
	}
	return strings.Join(candidates, ", ")
}

type Reply struct {
	ID      int64     `json:"id"`
	Author  User      `json:"author"`
	Body    string    `json:"body"`
	Created time.Time `json:"created_at"`
}
type Post struct {
	ID         int64     `json:"id"`
	Author     User      `json:"author"`
	Body       string    `json:"body"`
	Created    time.Time `json:"created_at"`
	Edited     bool      `json:"edited"`
	Revision   int64     `json:"revision"`
	Photos     []Photo   `json:"photos"`
	ReplyCount int       `json:"reply_count"`
	Saved      bool      `json:"saved"`
	Replies    []Reply   `json:"replies,omitempty"`
}

func (p Post) URL() string { return fmt.Sprintf("/posts/%d", p.ID) }

type PostPage struct {
	Posts  []Post `json:"posts"`
	Next   string `json:"next"`
	Anchor int64  `json:"anchor"`
}
type cursor struct {
	Before int64 `json:"b"`
	Anchor int64 `json:"a"`
}

func parseCursor(value string) (cursor, error) {
	if value == "" {
		return cursor{}, nil
	}
	if len(value) > 100 {
		return cursor{}, ErrInvalid
	}
	data, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return cursor{}, ErrInvalid
	}
	var c cursor
	if json.Unmarshal(data, &c) != nil || c.Before < 1 || c.Anchor < c.Before {
		return cursor{}, ErrInvalid
	}
	return c, nil
}
func encodeCursor(c cursor) string {
	data, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(data)
}

func fingerprint(parts ...[]byte) string {
	h := sha256.New()
	for _, p := range parts {
		binary.Write(h, binary.BigEndian, uint64(len(p)))
		h.Write(p)
	}
	return hex.EncodeToString(h.Sum(nil))
}
func postFingerprint(body string, photos []Photo) string {
	parts := [][]byte{[]byte(body)}
	for _, p := range photos {
		parts = append(parts, []byte(p.Alt), p.Data)
	}
	return fingerprint(parts...)
}

func (s *Store) CreatePost(ctx context.Context, author int64, body, key string, photos []Photo) (int64, error) {
	body = strings.TrimSpace(body)
	if !validText(body, MaxPostCharacters, true) || !validToken(key) || len(photos) > MaxPhotos || (body == "" && len(photos) == 0) {
		return 0, ErrInvalid
	}
	for _, p := range photos {
		if !validText(p.Alt, 1000, false) || len(p.Data) == 0 || len(p.Data) > 8<<20 || p.Width < 1 || p.Height < 1 || p.Width > 1920 || p.Height > 1920 {
			return 0, ErrInvalid
		}
	}
	print := postFingerprint(body, photos)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if err = requireUser(ctx, tx, author, false); err != nil {
		return 0, err
	}
	var id int64
	var previous string
	err = tx.QueryRowContext(ctx, "SELECT id,fingerprint FROM posts WHERE user_id=? AND submission_key=?", author, key).Scan(&id, &previous)
	if err == nil {
		if previous != print {
			return 0, ErrConflict
		}
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	if err = checkReceipt(ctx, tx, author, "post", key); err != nil {
		return 0, err
	}
	res, err := tx.ExecContext(ctx, "INSERT INTO posts(user_id,body,created_at,submission_key,fingerprint) VALUES(?,?,?,?,?)", author, body, time.Now().UnixMilli(), key, print)
	if err != nil {
		return 0, err
	}
	id, err = res.LastInsertId()
	if err != nil {
		return 0, err
	}
	for i, p := range photos {
		secret, err := newPhotoID()
		if err != nil {
			return 0, err
		}
		_, err = tx.ExecContext(ctx, "INSERT INTO photos(id,post_id,position,alt,data,width,height) VALUES(?,?,?,?,?,?,?)", secret, id, i, p.Alt, p.Data, p.Width, p.Height)
		if err != nil {
			return 0, err
		}
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO receipts VALUES(?,'post',?)", author, key); err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

const postSelect = `SELECT p.id,p.body,p.created_at,p.edited_at IS NOT NULL,p.revision,
 u.id,u.username,u.name,u.bio,u.role,u.disabled,
 (SELECT count(*) FROM replies r WHERE r.post_id=p.id),
 EXISTS(SELECT 1 FROM saved s WHERE s.post_id=p.id AND s.user_id=?)
 FROM posts p JOIN users u ON u.id=p.user_id `

func scanPost(row scanner) (Post, error) {
	var p Post
	var stamp int64
	err := row.Scan(&p.ID, &p.Body, &stamp, &p.Edited, &p.Revision, &p.Author.ID, &p.Author.Username, &p.Author.Name, &p.Author.Bio, &p.Author.Role, &p.Author.Disabled, &p.ReplyCount, &p.Saved)
	p.Created = time.UnixMilli(stamp)
	p.Photos = []Photo{}
	return p, err
}
func (s *Store) photos(ctx context.Context, id int64) ([]Photo, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id,alt,width,height FROM photos WHERE post_id=? ORDER BY position", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Photo{}
	for rows.Next() {
		var p Photo
		if err = rows.Scan(&p.ID, &p.Alt, &p.Width, &p.Height); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
func (s *Store) Posts(ctx context.Context, viewer int64, after string, saved bool, author int64) (PostPage, error) {
	c, err := parseCursor(after)
	if err != nil {
		return PostPage{}, err
	}
	if c.Anchor == 0 {
		if err = s.db.QueryRowContext(ctx, "SELECT coalesce(max(id),0) FROM posts").Scan(&c.Anchor); err != nil {
			return PostPage{}, err
		}
	}
	query := postSelect + " WHERE p.id<=? AND (?=0 OR p.id<?)"
	args := []any{viewer, c.Anchor, c.Before, c.Before}
	if saved {
		query += " AND EXISTS(SELECT 1 FROM saved v WHERE v.post_id=p.id AND v.user_id=?)"
		args = append(args, viewer)
	}
	if author > 0 {
		query += " AND p.user_id=?"
		args = append(args, author)
	}
	query += " ORDER BY p.id DESC LIMIT ?"
	args = append(args, PageSize+1)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return PostPage{}, err
	}
	out := PostPage{Posts: []Post{}, Anchor: c.Anchor}
	for rows.Next() {
		p, err := scanPost(rows)
		if err != nil {
			rows.Close()
			return out, err
		}
		out.Posts = append(out.Posts, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	if len(out.Posts) > PageSize {
		out.Posts = out.Posts[:PageSize]
		out.Next = encodeCursor(cursor{Before: out.Posts[len(out.Posts)-1].ID, Anchor: c.Anchor})
	}
	for i := range out.Posts {
		out.Posts[i].Photos, err = s.photos(ctx, out.Posts[i].ID)
		if err != nil {
			return out, err
		}
	}
	return out, nil
}
func (s *Store) Post(ctx context.Context, viewer, id int64) (Post, error) {
	p, err := scanPost(s.db.QueryRowContext(ctx, postSelect+" WHERE p.id=?", viewer, id))
	if err != nil {
		return p, err
	}
	p.Photos, err = s.photos(ctx, id)
	if err != nil {
		return p, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT r.id,r.body,r.created_at,u.id,u.username,u.name,u.bio,u.role,u.disabled FROM replies r JOIN users u ON u.id=r.user_id WHERE r.post_id=? ORDER BY r.id LIMIT 500`, id)
	if err != nil {
		return p, err
	}
	defer rows.Close()
	p.Replies = []Reply{}
	for rows.Next() {
		var v Reply
		var stamp int64
		err = rows.Scan(&v.ID, &v.Body, &stamp, &v.Author.ID, &v.Author.Username, &v.Author.Name, &v.Author.Bio, &v.Author.Role, &v.Author.Disabled)
		if err != nil {
			return p, err
		}
		v.Created = time.UnixMilli(stamp)
		p.Replies = append(p.Replies, v)
	}
	return p, rows.Err()
}
func (s *Store) Photo(ctx context.Context, id string) (Photo, error) {
	var p Photo
	err := s.db.QueryRowContext(ctx, "SELECT id,alt,width,height,data FROM photos WHERE id=?", id).Scan(&p.ID, &p.Alt, &p.Width, &p.Height, &p.Data)
	return p, err
}

func (s *Store) AddReply(ctx context.Context, author, post int64, body, key string) (int64, error) {
	body = strings.TrimSpace(body)
	if body == "" || !validText(body, 5000, true) || !validToken(key) {
		return 0, ErrInvalid
	}
	print := fingerprint([]byte(fmt.Sprint(post)), []byte(body))
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if err = requireUser(ctx, tx, author, false); err != nil {
		return 0, err
	}
	var id int64
	var previous string
	err = tx.QueryRowContext(ctx, "SELECT id,fingerprint FROM replies WHERE user_id=? AND submission_key=?", author, key).Scan(&id, &previous)
	if err == nil {
		if previous != print {
			return 0, ErrConflict
		}
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	var n int
	if err = checkReceipt(ctx, tx, author, "reply", key); err != nil {
		return 0, err
	}
	if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM posts WHERE id=?", post).Scan(&n); err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, sql.ErrNoRows
	}
	if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM replies WHERE post_id=?", post).Scan(&n); err != nil {
		return 0, err
	}
	if n >= 500 {
		return 0, errors.New("this post has reached its reply limit")
	}
	res, err := tx.ExecContext(ctx, "INSERT INTO replies(post_id,user_id,body,created_at,submission_key,fingerprint) VALUES(?,?,?,?,?,?)", post, author, body, time.Now().UnixMilli(), key, print)
	if err != nil {
		return 0, err
	}
	id, err = res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO receipts VALUES(?,'reply',?)", author, key); err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

func checkReceipt(ctx context.Context, tx *sql.Tx, user int64, kind, key string) error {
	var exists bool
	if err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM receipts WHERE user_id=? AND kind=? AND submission_key=?)", user, kind, key).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return ErrConflict
	}
	return nil
}
func (s *Store) SavePost(ctx context.Context, user, post int64, save bool) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = requireUser(ctx, tx, user, false); err != nil {
		return err
	}
	var n int
	if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM posts WHERE id=?", post).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	if save {
		_, err = tx.ExecContext(ctx, "INSERT OR IGNORE INTO saved VALUES(?,?)", user, post)
	} else {
		_, err = tx.ExecContext(ctx, "DELETE FROM saved WHERE user_id=? AND post_id=?", user, post)
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) EditPost(ctx context.Context, actor, id, revision int64, body string) error {
	body = strings.TrimSpace(body)
	if !validText(body, MaxPostCharacters, true) {
		return ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = requireUser(ctx, tx, actor, false); err != nil {
		return err
	}
	var owner int64
	var photos int
	if err = tx.QueryRowContext(ctx, "SELECT user_id,(SELECT count(*) FROM photos WHERE post_id=posts.id) FROM posts WHERE id=?", id).Scan(&owner, &photos); err != nil {
		return err
	}
	if owner != actor {
		return ErrForbidden
	}
	if body == "" && photos == 0 {
		return ErrInvalid
	}
	res, err := tx.ExecContext(ctx, "UPDATE posts SET body=?,edited_at=?,revision=revision+1 WHERE id=? AND revision=?", body, time.Now().UnixMilli(), id, revision)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return ErrConflict
	}
	return tx.Commit()
}
func (s *Store) DeleteContent(ctx context.Context, actor, id int64, reply bool) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = requireUser(ctx, tx, actor, false); err != nil {
		return err
	}
	table := "posts"
	if reply {
		table = "replies"
	}
	var author int64
	err = tx.QueryRowContext(ctx, "SELECT user_id FROM "+table+" WHERE id=?", id).Scan(&author)
	if err != nil {
		return err
	}
	if author != actor {
		if err = requireUser(ctx, tx, actor, true); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE id=?", id); err != nil {
		return err
	}
	return tx.Commit()
}
