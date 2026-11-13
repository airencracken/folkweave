// SPDX-License-Identifier: AGPL-3.0-or-later
package folkweave

import (
	"database/sql"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
)

func TestOwnerSetsSitePalette(t *testing.T) {
	s, a, owner := webFixture(t)
	owner.signIn(t, "owner")
	if body := owner.request("GET", "/settings", nil, nil).Body.String(); !strings.Contains(body, `<option value="sage" selected>Sage</option>`) {
		t.Fatal("settings should show the default palette selected")
	}
	if w := owner.form(t, "/settings", url.Values{"title": {"Kept"}, "about": {""}, "palette": {"slate"}}, false); w.Code != 303 {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, path := range []string{"/", "/settings", "/admin/metrics"} {
		if body := owner.request("GET", path, nil, nil).Body.String(); !strings.Contains(body, `data-palette="slate" data-palette-default="slate"`) {
			t.Fatal(path, "site palette not rendered")
		}
	}
	guest := &testClient{app: a, cookies: map[string]*http.Cookie{}}
	if body := guest.request("GET", "/login", nil, nil).Body.String(); !strings.Contains(body, `data-palette="slate"`) {
		t.Fatal("sign-in page ignores the site palette")
	}
	for _, bad := range []string{"", "Slate", "slate ", "purple", `sage" onload="x`, "sage slate", strings.Repeat("s", 4096)} {
		if w := owner.form(t, "/settings", url.Values{"title": {"Changed"}, "about": {""}, "palette": {bad}}, false); w.Code != 422 {
			t.Fatalf("palette %q accepted: %d", bad, w.Code)
		}
	}
	settings, err := s.Settings(testContext)
	if err != nil || settings.Palette != "slate" || settings.Title != "Kept" {
		t.Fatal("rejected palette changed settings", settings, err)
	}
	testMember(t, s, "member")
	member := &testClient{app: a, cookies: map[string]*http.Cookie{}}
	member.signIn(t, "member")
	if w := member.form(t, "/settings", url.Values{"title": {"Taken"}, "about": {""}, "palette": {"sage"}}, false); w.Code != 403 {
		t.Fatal("member changed site palette", w.Code)
	}
}

func TestOpenAddsPaletteColumnToOlderDatabases(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "data")
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec("ALTER TABLE settings DROP COLUMN palette; INSERT INTO settings(id,title,about) VALUES(1,'Old group','')"); err != nil {
		t.Fatal(err)
	}
	s.Close()
	for round := 0; round < 2; round++ {
		s, err = Open(dir)
		if err != nil {
			t.Fatal(round, err)
		}
		settings, err := s.Settings(testContext)
		if err != nil || settings.Title != "Old group" || settings.Palette != "" {
			t.Fatal(round, settings, err)
		}
		s.Close()
	}
}

func TestPaletteMigrationReportsFailure(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "x.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	if err = addSettingsPalette(testContext, db); err == nil {
		t.Fatal("closed database accepted")
	}
}
