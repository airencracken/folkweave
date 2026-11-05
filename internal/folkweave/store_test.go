package folkweave

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/airencracken/comfylib/token"
	"golang.org/x/crypto/bcrypt"
)

var testContext = context.Background()

func testStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	hash, _ := bcrypt.GenerateFromPassword([]byte("a-long-test-password"), bcrypt.MinCost)
	if err = s.Bootstrap(testContext, "owner", string(hash), "Owner", "The Test Folks"); err != nil {
		t.Fatal(err)
	}
	return s
}
func testMember(t *testing.T, s *Store, name string) int64 {
	t.Helper()
	invite, err := s.NewInvitation(testContext, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte("a-long-test-password"), bcrypt.MinCost)
	id, err := s.Register(testContext, name, string(hash), strings.ToUpper(name[:1])+name[1:], invite)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
func testKey() string { key, _ := token.New(); return key }
func testPhoto(t *testing.T) Photo {
	t.Helper()
	pic := image.NewRGBA(image.Rect(0, 0, 12, 8))
	pic.Set(0, 0, color.RGBA{230, 100, 40, 255})
	var data bytes.Buffer
	if err := jpeg.Encode(&data, pic, nil); err != nil {
		t.Fatal(err)
	}
	p, err := NormalizePhoto(data.Bytes(), "An orange corner")
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestSchemaConstraintsAndSecrets(t *testing.T) {
	s := testStore(t)
	if err := s.Check(testContext); err != nil {
		t.Fatal(err)
	}
	var enabled int
	s.db.QueryRow("PRAGMA foreign_keys").Scan(&enabled)
	if enabled != 1 {
		t.Fatal("foreign keys disabled")
	}
	if _, err := s.db.Exec("INSERT INTO photos VALUES('orphan',999,0,'',x'01',1,1)"); err == nil {
		t.Fatal("orphan attachment accepted")
	}
	if _, err := s.db.Exec("UPDATE users SET role='anything'"); err == nil {
		t.Fatal("invalid role accepted")
	}
	if err := s.Bootstrap(testContext, "other", "not-a-hash", "Other", "Other"); err == nil {
		t.Fatal("second bootstrap accepted")
	}
	secret, err := s.NewSession(testContext, 1)
	if err != nil {
		t.Fatal(err)
	}
	var stored string
	s.db.QueryRow("SELECT token_hash FROM sessions").Scan(&stored)
	if stored == secret || stored != token.Hash(secret) {
		t.Fatal("session secret stored in clear")
	}
	u, err := s.Session(testContext, token.Hash(secret))
	if err != nil || u.ID != 1 {
		t.Fatal("session missing", err)
	}
	s.db.Exec("UPDATE sessions SET expires_at=0")
	if _, err = s.Session(testContext, token.Hash(secret)); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("expired session accepted", err)
	}
}

func TestInvitationRedemptionAtomicAndConcurrent(t *testing.T) {
	s := testStore(t)
	invite, err := s.NewInvitation(testContext, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte("a-long-test-password"), bcrypt.MinCost)
	var wg sync.WaitGroup
	results := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := s.Register(testContext, fmt.Sprintf("member_%d", i), string(hash), "Member", invite)
			results <- err
		}(i)
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		} else if !errors.Is(err, ErrInvitation) {
			t.Fatal(err)
		}
	}
	if success != 1 {
		t.Fatalf("redemptions=%d", success)
	}
	invite, err = s.NewInvitation(testContext, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Register(testContext, "owner", string(hash), "Other", invite); !errors.Is(err, ErrUsername) {
		t.Fatal(err)
	}
	var uses int
	s.db.QueryRow("SELECT uses FROM invitations WHERE token_hash=?", token.Hash(invite)).Scan(&uses)
	if uses != 0 {
		t.Fatal("failed registration consumed an invitation")
	}
	s.db.Exec("UPDATE invitations SET expires_at=? WHERE token_hash=?", time.Now().UnixMilli(), token.Hash(invite))
	if _, err = s.Register(testContext, "late_member", string(hash), "Late", invite); !errors.Is(err, ErrInvitation) {
		t.Fatal("expiry boundary accepted", err)
	}
}

func TestRevokedInvitationAndMemberPermissions(t *testing.T) {
	s := testStore(t)
	member := testMember(t, s, "member")
	if _, err := s.NewInvitation(testContext, member, 1); !errors.Is(err, ErrForbidden) {
		t.Fatal("member could invite", err)
	}
	invite, _ := s.NewInvitation(testContext, 1, 1)
	var id int64
	s.db.QueryRow("SELECT id FROM invitations WHERE token_hash=?", token.Hash(invite)).Scan(&id)
	if err := s.RevokeInvitation(testContext, member, id); !errors.Is(err, ErrForbidden) {
		t.Fatal(err)
	}
	if err := s.RevokeInvitation(testContext, 1, id); err != nil {
		t.Fatal(err)
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte("a-long-test-password"), bcrypt.MinCost)
	if _, err := s.Register(testContext, "new_member", string(hash), "Name", invite); !errors.Is(err, ErrInvitation) {
		t.Fatal("revoked invite accepted", err)
	}
}

func TestPostRetryIdentityAndAtomicPhotos(t *testing.T) {
	s := testStore(t)
	key := testKey()
	photo := testPhoto(t)
	id, err := s.CreatePost(testContext, 1, "Morning walk", key, []Photo{photo, photo})
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.CreatePost(testContext, 1, "Morning walk", key, []Photo{photo, photo})
	if err != nil || again != id {
		t.Fatal("retry duplicated", again, err)
	}
	if _, err = s.CreatePost(testContext, 1, "Changed", key, []Photo{photo}); !errors.Is(err, ErrConflict) {
		t.Fatal("key reused for changed content", err)
	}
	var count int
	s.db.QueryRow("SELECT count(*) FROM posts").Scan(&count)
	if count != 1 {
		t.Fatal("duplicate post")
	}
	if _, err = s.db.Exec(`CREATE TRIGGER fail_second_photo BEFORE INSERT ON photos WHEN NEW.position=1 BEGIN SELECT RAISE(ABORT,'injected write failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreatePost(testContext, 1, "Must roll back", testKey(), []Photo{photo, photo}); err == nil {
		t.Fatal("injected failure ignored")
	}
	s.db.QueryRow("SELECT count(*) FROM posts").Scan(&count)
	if count != 1 {
		t.Fatal("partial post survived")
	}
	s.db.QueryRow("SELECT count(*) FROM photos").Scan(&count)
	if count != 2 {
		t.Fatal("partial photos survived")
	}
}

func TestPaginationPropertyWithConcurrentInsertAndEdit(t *testing.T) {
	s := testStore(t)
	for i := 0; i < 67; i++ {
		if _, err := s.CreatePost(testContext, 1, fmt.Sprint(i), testKey(), nil); err != nil {
			t.Fatal(err)
		}
	}
	page, err := s.Posts(testContext, 1, "", false, 0)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[int64]bool{}
	anchor := page.Anchor
	rng := rand.New(rand.NewSource(31))
	for {
		for _, p := range page.Posts {
			if seen[p.ID] || p.ID > anchor {
				t.Fatal("unstable page", p.ID)
			}
			seen[p.ID] = true
		}
		if page.Next == "" {
			break
		}
		for i := 0; i < rng.Intn(5)+1; i++ {
			s.CreatePost(testContext, 1, "New arrival", testKey(), nil)
		}
		if err = s.EditPost(testContext, 1, 1, 1, "Edited old post"); err != nil && !errors.Is(err, ErrConflict) {
			t.Fatal(err)
		}
		page, err = s.Posts(testContext, 1, page.Next, false, 0)
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(seen) != 67 {
		t.Fatalf("visited=%d want 67", len(seen))
	}
	for _, bad := range []string{"?", "-1", strings.Repeat("A", 200), encodeCursor(cursor{Before: 2, Anchor: 1})} {
		if _, err = s.Posts(testContext, 1, bad, false, 0); !errors.Is(err, ErrInvalid) {
			t.Fatal("bad cursor accepted", bad)
		}
	}
}

func TestRepliesSaveDeleteAndSuspension(t *testing.T) {
	s := testStore(t)
	member := testMember(t, s, "member")
	other := testMember(t, s, "other")
	post, err := s.CreatePost(testContext, member, "Hello", testKey(), []Photo{testPhoto(t)})
	if err != nil {
		t.Fatal(err)
	}
	key := testKey()
	reply, err := s.AddReply(testContext, other, post, "Welcome", key)
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.AddReply(testContext, other, post, "Welcome", key)
	if err != nil || reply != again {
		t.Fatal("duplicate reply", err)
	}
	if _, err = s.AddReply(testContext, other, post, "Changed", key); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	if err = s.SavePost(testContext, other, post, true); err != nil {
		t.Fatal(err)
	}
	page, _ := s.Posts(testContext, member, "", true, 0)
	if len(page.Posts) != 0 {
		t.Fatal("bookmarks leaked")
	}
	if err = s.EditPost(testContext, other, post, 1, "stolen"); !errors.Is(err, ErrForbidden) {
		t.Fatal(err)
	}
	if err = s.DeleteContent(testContext, other, post, false); !errors.Is(err, ErrForbidden) {
		t.Fatal(err)
	}
	secret, _ := s.NewSession(testContext, member)
	if err = s.SetDisabled(testContext, 1, member, true); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Session(testContext, token.Hash(secret)); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("suspended session survived")
	}
	if _, err = s.CreatePost(testContext, member, "Forbidden", testKey(), nil); !errors.Is(err, ErrForbidden) {
		t.Fatal(err)
	}
	if err = s.SetDisabled(testContext, 1, 1, true); !errors.Is(err, ErrForbidden) {
		t.Fatal("owner suspended")
	}
	if err = s.DeleteContent(testContext, 1, post, false); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"photos", "replies", "saved"} {
		var n int
		s.db.QueryRow("SELECT count(*) FROM " + table).Scan(&n)
		if n != 0 {
			t.Fatal("orphan rows in", table)
		}
	}
}

func TestTextAndPhotoValidation(t *testing.T) {
	s := testStore(t)
	for _, body := range []string{"", strings.Repeat("x", 10001), "null\x00body", string([]byte{0xff})} {
		if _, err := s.CreatePost(testContext, 1, body, testKey(), nil); !errors.Is(err, ErrInvalid) {
			t.Fatalf("accepted %q", body[:min(len(body), 30)])
		}
	}
	if _, err := s.CreatePost(testContext, 1, strings.Repeat("界", 10000), testKey(), nil); err != nil {
		t.Fatal("Unicode boundary", err)
	}
	if _, err := s.CreatePost(testContext, 999, "Missing actor", testKey(), nil); !errors.Is(err, ErrForbidden) {
		t.Fatal(err)
	}
	for _, p := range []Photo{{Alt: "invalid", Data: []byte{1}}, {Alt: "\x00", Data: []byte{1}, Width: 1, Height: 1}} {
		if _, err := s.CreatePost(testContext, 1, "text", testKey(), []Photo{p}); err == nil {
			t.Fatal("invalid photo accepted")
		}
	}
}

func TestVerifiedSnapshotRestoresPhotosAndSchema(t *testing.T) {
	s := testStore(t)
	id, err := s.CreatePost(testContext, 1, "A copy worth keeping", testKey(), []Photo{testPhoto(t)})
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "snapshot")
	if err = s.Snapshot(testContext, destination, 32<<20); err != nil {
		t.Fatal(err)
	}
	copy, err := Open(destination)
	if err != nil {
		t.Fatal(err)
	}
	defer copy.Close()
	if err = copy.Check(testContext); err != nil {
		t.Fatal(err)
	}
	post, err := copy.Post(testContext, 1, id)
	if err != nil || len(post.Photos) != 1 {
		t.Fatal("photo link lost", err)
	}
	photo, err := copy.Photo(testContext, post.Photos[0].ID)
	if err != nil || len(photo.Data) == 0 {
		t.Fatal("photo content lost", err)
	}
	if err = s.Snapshot(testContext, filepath.Join(t.TempDir(), "small"), 1); err == nil {
		t.Fatal("snapshot limit ignored")
	}
}

func TestPublicJSONDoesNotExposeCredentialsOrPhotoBytes(t *testing.T) {
	s := testStore(t)
	id, _ := s.CreatePost(testContext, 1, "Hello", testKey(), []Photo{testPhoto(t)})
	p, err := s.Post(testContext, 1, id)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"password_hash", "fingerprint", "submission_key", "Data", "token_hash"} {
		if bytes.Contains(data, []byte(secret)) {
			t.Fatal("private data in API", secret)
		}
	}
}

func TestDeletedSubmissionsCannotBeResurrectedByRetry(t *testing.T) {
	s := testStore(t)
	key := testKey()
	id, err := s.CreatePost(testContext, 1, "A post to remove", key, nil)
	if err != nil {
		t.Fatal(err)
	}
	replyKey := testKey()
	reply, err := s.AddReply(testContext, 1, id, "A reply to remove", replyKey)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.DeleteContent(testContext, 1, reply, true); err != nil {
		t.Fatal(err)
	}
	if _, err = s.AddReply(testContext, 1, id, "A reply to remove", replyKey); !errors.Is(err, ErrConflict) {
		t.Fatal("deleted reply resurrected", err)
	}
	if err = s.DeleteContent(testContext, 1, id, false); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreatePost(testContext, 1, "A post to remove", key, nil); !errors.Is(err, ErrConflict) {
		t.Fatal("deleted post resurrected", err)
	}
}
func TestPasswordRecoveryRevokesSessionsAndSuspensionCanBeLifted(t *testing.T) {
	s := testStore(t)
	member := testMember(t, s, "member")
	session, err := s.NewSession(testContext, member)
	if err != nil {
		t.Fatal(err)
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte("a-new-private-password"), bcrypt.MinCost)
	if err = s.SetPassword(testContext, "member", string(hash)); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Session(testContext, token.Hash(session)); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("old session survived password recovery")
	}
	_, stored, err := s.Credentials(testContext, "member")
	if err != nil || bcrypt.CompareHashAndPassword([]byte(stored), []byte("a-new-private-password")) != nil {
		t.Fatal("recovery did not change password")
	}
	if err = s.SetDisabled(testContext, 1, member, true); err != nil {
		t.Fatal(err)
	}
	if err = s.Profile(testContext, member, "Hidden update", ""); !errors.Is(err, ErrForbidden) {
		t.Fatal("suspended profile write accepted")
	}
	people, err := s.People(testContext, true)
	if err != nil || len(people) != 2 {
		t.Fatal("owner cannot find suspended member")
	}
	active, _ := s.People(testContext)
	if len(active) != 1 {
		t.Fatal("suspended member remains active")
	}
	if err = s.SetDisabled(testContext, 1, member, false); err != nil {
		t.Fatal(err)
	}
	if _, err = s.NewSession(testContext, member); err != nil {
		t.Fatal("restored member cannot sign in", err)
	}
}

func TestPrivateDataPathsAndUnsupportedSchema(t *testing.T) {
	root := t.TempDir()
	public := filepath.Join(root, "public")
	os.Mkdir(public, 0755)
	if _, err := Open(public); err == nil {
		t.Fatal("public data directory accepted")
	}
	target := filepath.Join(root, "target")
	os.Mkdir(target, 0700)
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(target, alias); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(alias); err == nil {
		t.Fatal("symlinked data directory accepted")
	}
	data := filepath.Join(root, "data")
	s, err := Open(data)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	path := filepath.Join(data, "folkweave.db")
	if err = os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err = Open(data); err == nil {
		t.Fatal("world-readable database accepted")
	}
	os.Chmod(path, 0600)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("DROP TABLE schema_version; CREATE TABLE schema_version(version INTEGER PRIMARY KEY); INSERT INTO schema_version VALUES(999)"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if _, err = Open(data); err == nil {
		t.Fatal("future schema silently opened")
	}
}
