// SPDX-License-Identifier: AGPL-3.0-or-later

package folkweave

import (
	"bytes"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/airencracken/comfylib/totp"
	"github.com/airencracken/comfylib/twofactor"
	"github.com/airencracken/comfylib/twofactor/factortest"
)

func TestAuthenticatorSignIn(t *testing.T) {
	t.Setenv("FOLKWEAVE_TOTP_KEY", strings.Repeat("ab", 32))
	_, app, _ := webFixture(t)
	factortest.Run(t, factortest.Config{
		Handler: app, Origin: "http://example.test",
		Username: "owner", Password: "a-long-test-password", Home: "/",
	})
}

func TestAuthenticatorRecoveryIsAtomicAndRequiresItsOriginalKey(t *testing.T) {
	t.Setenv("FOLKWEAVE_TOTP_KEY", strings.Repeat("ab", 32))
	store, app, owner := webFixture(t)
	owner.signIn(t, "owner")
	w := owner.form(t, twofactor.AccountPath, url.Values{"action": {"start"}, "password": {"a-long-test-password"}}, false)
	if w.Code != http.StatusSeeOther {
		t.Fatal("could not start enrollment", w.Code)
	}
	w = owner.request("GET", twofactor.AccountPath, nil, nil)
	secret := regexp.MustCompile(`id="authenticator-key">([^<]+)`).FindStringSubmatch(w.Body.String())
	if len(secret) != 2 {
		t.Fatal("setup key missing")
	}
	code, err := totp.Code(secret[1], time.Now())
	if err != nil {
		t.Fatal(err)
	}
	w = owner.form(t, twofactor.AccountPath, url.Values{"action": {"enable"}, "code": {code}}, false)
	recovery := regexp.MustCompile(`<li><code>([A-Z2-9]{5}-[A-Z2-9]{5})</code></li>`).FindStringSubmatch(w.Body.String())
	if w.Code != http.StatusOK || len(recovery) != 2 {
		t.Fatal("could not enroll", w.Code)
	}
	var encrypted []byte
	if err := store.db.QueryRow("SELECT ciphertext FROM twofactor_secrets WHERE user_id=1").Scan(&encrypted); err != nil ||
		bytes.Contains(encrypted, []byte(totp.Normalise(secret[1]))) {
		t.Fatalf("authenticator secret was not safely persisted: %v", err)
	}
	missing := Config{BaseURL: "http://example.test"}
	for _, key := range []string{"", strings.Repeat("cd", 32)} {
		t.Setenv("FOLKWEAVE_TOTP_KEY", key)
		if _, err := NewApp(store, missing); err == nil {
			t.Fatal("enrolled instance started without its key")
		}
	}

	guest := &testClient{app: app, cookies: map[string]*http.Cookie{}}
	guest.signIn(t, "owner")
	if guest.cookies[app.cookieName("session")] != nil {
		t.Fatal("first factor issued a full session")
	}
	if _, err := store.db.Exec(`CREATE TRIGGER reject_session BEFORE INSERT ON sessions
BEGIN SELECT RAISE(ABORT,'fixture failure'); END`); err != nil {
		t.Fatal(err)
	}
	if w = guest.form(t, twofactor.LoginPath, url.Values{"code": {recovery[1]}}, false); w.Code != http.StatusInternalServerError {
		t.Fatal("injected session error did not stop sign-in", w.Code)
	}
	if _, err := store.db.Exec("DROP TRIGGER reject_session"); err != nil {
		t.Fatal(err)
	}
	if w = guest.form(t, twofactor.LoginPath, url.Values{"code": {recovery[1]}}, false); w.Code != http.StatusSeeOther ||
		w.Header().Get("Location") != "/" || guest.cookies[app.cookieName("session")] == nil {
		t.Fatal("failed session creation consumed the recovery code", w.Code)
	}
}
