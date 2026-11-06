package folkweave

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/airencracken/comfylib/token"
	"golang.org/x/crypto/bcrypt"
)

func ownerHash(t *testing.T) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte("a-new-owner-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	return string(hash)
}
func TestCreateOwnerPreservesExistingGathering(t *testing.T) {
	s := testStore(t)
	hash := ownerHash(t)
	memberID := testMember(t, s, "member")
	secret, err := s.NewSession(testContext, memberID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec("UPDATE settings SET about='Keep our description'"); err != nil {
		t.Fatal(err)
	}
	beforeSettings, err := s.Settings(testContext)
	if err != nil {
		t.Fatal(err)
	}
	beforeMember, beforeHash, err := s.Credentials(testContext, "member")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.CreateOwner(testContext, " Second_Owner ", hash, "Second Owner", "Unused new title"); err != nil {
		t.Fatal(err)
	}
	u, stored, err := s.Credentials(testContext, "second_owner")
	if err != nil || !u.Owner() || u.Name != "Second Owner" || stored != hash {
		t.Fatal(u, err)
	}
	afterSettings, _ := s.Settings(testContext)
	if afterSettings != beforeSettings {
		t.Fatal("settings changed", afterSettings)
	}
	for _, existing := range []string{"member", "owner", "second_owner"} {
		if err = s.CreateOwner(testContext, existing, hash, "Replacement", "Other title"); !errors.Is(err, ErrUsername) {
			t.Fatal("existing account accepted", existing, err)
		}
	}
	afterMember, afterHash, _ := s.Credentials(testContext, "member")
	if afterMember != beforeMember || afterHash != beforeHash || afterMember.Role != "member" {
		t.Fatal("existing account changed")
	}
	if _, err = s.Session(testContext, token.Hash(secret)); err != nil {
		t.Fatal("existing session revoked", err)
	}
	if err = s.Bootstrap(testContext, "third", hash, "Third", "Other title"); !errors.Is(err, ErrConflict) {
		t.Fatal("init overwrote existing gathering", err)
	}
	if err = s.Check(testContext); err != nil {
		t.Fatal(err)
	}
}
func TestCreateFirstOwnerAndSettingsAreAtomic(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	hash := ownerHash(t)
	if _, err = s.db.Exec("CREATE TRIGGER fail_settings BEFORE INSERT ON settings BEGIN SELECT RAISE(ABORT,'injected settings failure');END"); err != nil {
		t.Fatal(err)
	}
	if err = s.CreateOwner(testContext, "first", hash, "First Owner", "Our gathering"); err == nil {
		t.Fatal("failure not injected")
	}
	var n int
	if err = s.db.QueryRow("SELECT count(*) FROM users").Scan(&n); err != nil || n != 0 {
		t.Fatal("partial owner committed", n, err)
	}
	if s.Ready(testContext) {
		t.Fatal("failed initializer reports ready")
	}
	s.db.Exec("DROP TRIGGER fail_settings")
	if err = s.CreateOwner(testContext, "first", hash, "First Owner", "Our gathering"); err != nil {
		t.Fatal(err)
	}
	settings, err := s.Settings(testContext)
	if err != nil || settings.Title != "Our gathering" || !s.Ready(testContext) {
		t.Fatal(settings, err)
	}
}
func TestCreateOwnerValidationAndProperty(t *testing.T) {
	s := testStore(t)
	hash := ownerHash(t)
	cases := []struct{ username, hash, name, title string }{{"bad; DROP TABLE users", hash, "Name", "Title"}, {"good", "not-a-hash", "Name", "Title"}, {"good", hash, "", "Title"}, {"good", hash, "bad\nname", "Title"}, {"good", hash, "Name", ""}, {"good", hash, "Name", strings.Repeat("x", 101)}}
	for _, tc := range cases {
		if err := s.CreateOwner(testContext, tc.username, tc.hash, tc.name, tc.title); !errors.Is(err, ErrInvalid) {
			t.Fatal("invalid owner accepted", tc.username, err)
		}
	}
	for i := range 20 {
		name := fmt.Sprintf("OWNER_%d", i)
		if err := s.CreateOwner(testContext, " "+name+" ", hash, "Owner é日", "Preserve original"); err != nil {
			t.Fatal(err)
		}
		u, _, err := s.Credentials(testContext, strings.ToLower(name))
		if err != nil || !u.Owner() || u.Username != strings.ToLower(name) {
			t.Fatal("normalization or role invariant failed", u, err)
		}
	}
	var count int
	s.db.QueryRow("SELECT count(*) FROM users").Scan(&count)
	if count != 21 {
		t.Fatal("invalid inputs modified accounts", count)
	}
}
func TestConcurrentOwnerCreationHasOneWinner(t *testing.T) {
	s := testStore(t)
	other, err := Open(filepath.Dir(s.Path))
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	hash := ownerHash(t)
	results := make(chan error, 8)
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			store := s
			if i%2 == 1 {
				store = other
			}
			results <- store.CreateOwner(testContext, "shared_owner", hash, "Owner", "Keep title")
		}()
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		} else if !errors.Is(err, ErrUsername) {
			t.Fatal(err)
		}
	}
	if success != 1 {
		t.Fatal("owner creation was not atomic", success)
	}
}
func TestCreatedOwnerCanSignInAndReachOwnerRoutes(t *testing.T) {
	s, _, client := webFixture(t)
	if err := s.CreateOwner(testContext, "new_owner", ownerHash(t), "New Owner", "Keep title"); err != nil {
		t.Fatal(err)
	}
	w := client.request("POST", "/login", url.Values{"username": {"new_owner"}, "password": {"a-new-owner-password"}, "csrf": {client.csrf(t)}}, nil)
	if w.Code != 303 {
		t.Fatal(w.Code, w.Body)
	}
	for _, path := range []string{"/settings", "/invitations", "/people"} {
		if w = client.request("GET", path, nil, nil); w.Code != 200 {
			t.Fatal(path, w.Code, w.Body)
		}
	}
}
