// SPDX-License-Identifier: AGPL-3.0-or-later
package folkweave

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestActivityCountsAuthorizedOpensAndSuccessfulLogins(t *testing.T) {
	s, a, owner := webFixture(t)
	owner.signIn(t, "owner")
	id, err := s.CreatePost(t.Context(), 1, "Activity fixture", testKey(), []Photo{{Data: previewFixture(t), Width: 1920, Height: 1280}})
	if err != nil {
		t.Fatal(err)
	}
	post, err := s.Post(t.Context(), 1, id)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{fmt.Sprintf("/posts/%d", id), fmt.Sprintf("/posts/%d", id), post.Photos[0].URL() + "?width=320"} {
		if w := owner.request("GET", path, nil, nil); w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	for i := 0; i < 2; i++ {
		if w := owner.request("GET", post.Photos[0].URL(), nil, nil); w.Code != 200 {
			t.Fatal(w.Code)
		}
	}
	owner.request("GET", fmt.Sprintf("/posts/%04d", id), nil, nil)
	owner.request("HEAD", fmt.Sprintf("/posts/%d", id), nil, nil)
	owner.request("GET", "/posts/99999999", nil, nil)
	owner.request("GET", "/", nil, nil)
	result, err := s.Analytics(t.Context(), time.Now())
	if err != nil || result.Views != 2 || result.Logins != 1 || len(result.LoginsByAccount) != 1 {
		t.Fatal(result, err)
	}
	guest := &testClient{app: a, cookies: map[string]*http.Cookie{}}
	if w := guest.form(t, "/login", url.Values{"username": {"owner"}, "password": {"wrong-password"}}, false); w.Code != 422 {
		t.Fatal(w.Code)
	}
	result, err = s.Analytics(t.Context(), time.Now())
	if err != nil || result.Logins != 1 {
		t.Fatal("failed login counted", result, err)
	}
	if w := owner.request("GET", "/admin/metrics", nil, nil); w.Code != 200 || !strings.Contains(w.Body.String(), "Recent sign-ins by account") || !strings.Contains(w.Body.String(), "comfy-admin-dashboard") {
		t.Fatal(w.Code, w.Body.String())
	}
	if _, err = s.db.Exec("UPDATE users SET disabled=1 WHERE id=1"); err != nil {
		t.Fatal(err)
	}
	if w := owner.request("GET", "/admin/metrics?role=admin", nil, nil); w.Code == 200 || strings.Contains(w.Body.String(), "Recent sign-ins") {
		t.Fatal("disabled admin read metrics", w.Code)
	}
}
func TestActivityLoginRollbackOnStorageFailure(t *testing.T) {
	s, _, _ := webFixture(t)
	if _, err := s.db.Exec("ALTER TABLE comfy_analytics_activity RENAME TO broken_activity"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.NewSession(t.Context(), 1); err == nil {
		t.Fatal("partial login accepted")
	}
	var n int
	if err := s.db.QueryRow("SELECT count(*) FROM sessions").Scan(&n); err != nil || n != 0 {
		t.Fatal("session escaped failed transaction", n, err)
	}
}
