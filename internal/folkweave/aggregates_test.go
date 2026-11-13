// SPDX-License-Identifier: AGPL-3.0-or-later
package folkweave

import (
	"bytes"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/airencracken/comfylib/analytics"
)

func activityTables(t *testing.T, db *sql.DB) int {
	t.Helper()
	var n int
	if err := db.QueryRow("SELECT count(*) FROM sqlite_schema WHERE name LIKE 'comfy_analytics%'").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// An installation upgraded from 0.5.0 or 0.6.0 holds per-account sign-ins and
// per-item views. Opening it removes them and keeps every other table intact.
func TestOpenRemovesStoredPerAccountAndPerItemActivity(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "data")
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Bootstrap(testContext, "owner", "$2a$04$abcdefghijklmnopqrstuuJ8m4Yb2nI0p6o3eY5w6f1Z9GjB4y8yK", "Owner", "Kept group"); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err = analytics.Initialize(testContext, s.db, now); err != nil {
		t.Fatal(err)
	}
	marker := "private-item-marker-7f3a"
	if err = analytics.RecordLogin(testContext, s.db, "1", now); err != nil {
		t.Fatal(err)
	}
	if err = analytics.RecordView(testContext, s.db, analytics.Post, "1", marker, now); err != nil {
		t.Fatal(err)
	}
	if activityTables(t, s.db) == 0 {
		t.Fatal("fixture did not create legacy activity tables")
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	for round := 0; round < 2; round++ {
		s, err = Open(dir)
		if err != nil {
			t.Fatal("reopen", round, err)
		}
		if n := activityTables(t, s.db); n != 0 {
			t.Fatal("legacy activity survived migration", round, n)
		}
		settings, err := s.Settings(testContext)
		if err != nil || settings.Title != "Kept group" {
			t.Fatal("migration changed application data", settings, err)
		}
		if _, err = s.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
			t.Fatal(err)
		}
		if err = s.Close(); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := os.ReadFile(filepath.Join(dir, "folkweave.db"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte(marker)) {
		t.Fatal("removed item identifier remains in freed database pages")
	}
}

func TestRetireActivityFailureKeepsDatabaseClosedAndReported(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "x.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	if err = retireActivity(testContext, db); err == nil {
		t.Fatal("closed database accepted")
	}
}

// Sign-ins, post opens and photo opens must not create activity storage.
func TestSignInsAndOpensAreNotRecorded(t *testing.T) {
	s, a, owner := webFixture(t)
	owner.signIn(t, "owner")
	id, err := s.CreatePost(t.Context(), 1, "Aggregate fixture", testKey(), []Photo{{Data: previewFixture(t), Width: 1920, Height: 1280}})
	if err != nil {
		t.Fatal(err)
	}
	post, err := s.Post(t.Context(), 1, id)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{fmt.Sprintf("/posts/%d", id), post.Photos[0].URL()} {
		if w := owner.request("GET", path, nil, nil); w.Code != 200 {
			t.Fatal(path, w.Code)
		}
	}
	request := httptest.NewRequest("GET", "http://example.test/auth/shared/callback", nil)
	if _, err = a.sharedSignIn(httptest.NewRecorder(), request, 1); err != nil {
		t.Fatal(err)
	}
	if n := activityTables(t, s.db); n != 0 {
		t.Fatal("activity storage created", n)
	}
	w := owner.request("GET", "/admin/metrics", nil, nil)
	body := w.Body.String()
	if w.Code != 200 || !strings.Contains(body, "Instance overview") || !strings.Contains(body, "Open invitations") {
		t.Fatal(w.Code, body)
	}
	for _, leaked := range []string{"Recent sign-ins by account", "Most viewed content", "Content views", "Aggregate fixture", ">owner<"} {
		if strings.Contains(body, leaked) {
			t.Fatal("dashboard shows per-account or per-item data:", leaked)
		}
	}
}

func TestSharedSignInRefusesInactiveAccounts(t *testing.T) {
	s, a, _ := webFixture(t)
	request := httptest.NewRequest("GET", "http://example.test/auth/shared/callback", nil)
	if _, err := s.db.Exec("UPDATE users SET disabled=1 WHERE id=1"); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	if _, err := a.sharedSignIn(response, request, 1); err == nil {
		t.Fatal("inactive account signed in")
	}
	if len(response.Result().Cookies()) != 0 {
		t.Fatal("inactive account received a session")
	}
}

func TestDisabledOwnerCannotReadMetrics(t *testing.T) {
	s, _, owner := webFixture(t)
	owner.signIn(t, "owner")
	if _, err := s.db.Exec("UPDATE users SET disabled=1 WHERE id=1"); err != nil {
		t.Fatal(err)
	}
	if w := owner.request("GET", "/admin/metrics?role=admin", nil, nil); w.Code == http.StatusOK || strings.Contains(w.Body.String(), "Instance overview") {
		t.Fatal("disabled owner read metrics", w.Code)
	}
}

func TestOpenInvitationMetricCountsOnlyRedeemableInvitations(t *testing.T) {
	s := testStore(t)
	for i := 0; i < 3; i++ {
		if _, err := s.NewInvitation(testContext, 1, 2); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.db.Exec("UPDATE invitations SET revoked=1 WHERE id=1; UPDATE invitations SET expires_at=1 WHERE id=2"); err != nil {
		t.Fatal(err)
	}
	metrics, err := s.Metrics(testContext, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range metrics {
		if m.Label == "Open invitations" && m.Value != 1 {
			t.Fatal("open invitations", m.Value)
		}
	}
}
