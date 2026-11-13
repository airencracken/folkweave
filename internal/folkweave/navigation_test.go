// SPDX-License-Identifier: AGPL-3.0-or-later
package folkweave

import (
	"strings"
	"testing"
)

// Each owner destination has one home: Admin in the main navigation, with
// Settings, Invitations and Metrics as its sub-navigation. The Account menu
// holds personal items only, and the footer holds no navigation.
func TestEachDestinationHasOneHome(t *testing.T) {
	s, _, owner := webFixture(t)
	if _, err := s.CreatePost(testContext, 1, "Footer fixture", testKey(), nil); err != nil {
		t.Fatal(err)
	}
	owner.signIn(t, "owner")
	for _, path := range []string{"/", "/people", "/account", "/settings", "/invitations", "/admin/metrics"} {
		w := owner.request("GET", path, nil, nil)
		if w.Code != 200 {
			t.Fatal(path, w.Code)
		}
		body := w.Body.String()
		header := strings.SplitN(body, "</header>", 2)[0]
		footer := body[strings.LastIndex(body, "<footer class=\"site-footer\">"):]
		menu := header[strings.Index(header, `class="comfy-menu comfy-account"`):]
		if strings.Count(header, `href="/settings"`) != 1 || strings.Contains(menu, `href="/settings"`) || strings.Contains(menu, `href="/invitations"`) {
			t.Fatal(path, "owner settings reachable from more than the Admin link")
		}
		if strings.Count(body, `action="/logout"`) != 1 || strings.Count(body, `href="/account/export"`) > 1 {
			t.Fatal(path, "sign out or export repeated")
		}
		for _, link := range []string{"/settings", "/account/export", "/logout"} {
			if strings.Contains(footer, link) {
				t.Fatal(path, "footer repeats navigation", link)
			}
		}
		if !strings.Contains(menu, `data-theme-select`) {
			t.Fatal(path, "theme picker missing from the Account menu")
		}
		admin := path == "/settings" || path == "/invitations" || path == "/admin/metrics"
		if strings.Contains(body, `aria-label="Administration"`) != admin {
			t.Fatal(path, "admin sub-navigation placement")
		}
		if admin {
			nav := body[strings.Index(body, `aria-label="Administration"`):]
			nav = nav[:strings.Index(nav, "</nav>")]
			for _, link := range []string{`href="/settings"`, `href="/invitations"`, `href="/admin/metrics"`} {
				if !strings.Contains(nav, link) {
					t.Fatal(path, "admin sub-navigation missing", link)
				}
			}
			if strings.Count(nav, `aria-current="page"`) != 1 || !strings.Contains(header, `aria-current="page">Admin</a>`) {
				t.Fatal(path, "current admin page not marked")
			}
		}
	}
	settings := owner.request("GET", "/settings", nil, nil).Body.String()
	if strings.Contains(settings, "Instance metrics and health") {
		t.Fatal("stray metrics paragraph link remains")
	}
	timeline := owner.request("GET", "/", nil, nil).Body.String()
	if strings.Contains(timeline, "Everyone here") || strings.Count(timeline, "Sharing with <strong>everyone here</strong>") != 1 {
		t.Fatal("post footers should not repeat the audience; the composer states it once")
	}
}

func TestTimesAndDatesFollowsYourProfile(t *testing.T) {
	_, _, owner := webFixture(t)
	owner.signIn(t, "owner")
	body := owner.request("GET", "/account", nil, nil).Body.String()
	heading := strings.Index(body, "<h1>Account</h1>")
	visible := strings.Index(body, "Your profile is visible to all members.")
	profile := strings.Index(body, "<h2>Your profile</h2>")
	times := strings.Index(body, "<h2>Times and dates</h2>")
	export := strings.Index(body, "<h2>Export your data</h2>")
	if heading < 0 || !(heading < visible && visible < profile && profile < times && times < export) {
		t.Fatal("account order", heading, visible, profile, times, export)
	}
}

func TestSignedOutPagesOfferThemeWithoutAccountMenu(t *testing.T) {
	_, _, guest := webFixture(t)
	body := guest.request("GET", "/login", nil, nil).Body.String()
	if !strings.Contains(body, `data-theme-select`) || strings.Contains(body, "comfy-account") {
		t.Fatal("signed-out header")
	}
	head := body[:strings.Index(body, "</head>")]
	theme, css := strings.Index(head, `/static/theme.js"></script>`), strings.Index(head, `rel="stylesheet"`)
	if theme < 0 || theme > css || strings.Contains(head, `theme.js" defer`) {
		t.Fatal("theme script must run before styles to avoid a flash")
	}
	if strings.Index(head, "/static/app.css") < strings.Index(head, "/static/comfyware/components.css") {
		t.Fatal("app styles must load after shared styles")
	}
}

func TestStaticThemeAndStyleAssets(t *testing.T) {
	_, _, guest := webFixture(t)
	for path, want := range map[string]string{"/static/theme.js": "data-theme", "/static/app.css": "--gold"} {
		w := guest.request("GET", path, nil, nil)
		if w.Code != 200 || !strings.Contains(w.Body.String(), want) {
			t.Fatal(path, w.Code)
		}
	}
	if w := guest.request("GET", "/static/theme.test.cjs", nil, nil); w.Code != 404 {
		t.Fatal("test script served", w.Code)
	}
}
