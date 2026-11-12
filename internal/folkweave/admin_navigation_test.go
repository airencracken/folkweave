package folkweave

import (
	"net/url"
	"strings"
	"testing"
)

func TestOwnerHasVisibleAdminNavigationAndMembersCannotOpenSettings(t *testing.T) {
	store, _, client := webFixture(t)
	testMember(t, store, "member")
	client.signIn(t, "owner")
	for _, path := range []string{"/", "/saved", "/people", "/account", "/settings"} {
		response := client.request("GET", path, nil, nil)
		header := strings.Split(response.Body.String(), "</header>")[0]
		if response.Code != 200 || !strings.Contains(header, `href="/settings"`) || !strings.Contains(header, `>Admin</a>`) {
			t.Fatalf("owner cannot discover administration from %s: %d", path, response.Code)
		}
	}
	client.form(t, "/logout", url.Values{}, false)
	client.signIn(t, "member")
	if body := client.request("GET", "/", nil, nil).Body.String(); strings.Contains(body, `>Admin</a>`) {
		t.Fatal("member sees owner navigation")
	}
	for _, path := range []string{"/settings", "/settings?owner=true", "/settings?role=admin"} {
		if response := client.request("GET", path, nil, nil); response.Code != 403 {
			t.Fatal("member opened settings", path, response.Code)
		}
	}
}
