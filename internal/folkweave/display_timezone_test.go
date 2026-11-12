package folkweave

import (
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestDisplayTimezoneAccountRoutesAndRendering(t *testing.T) {
	s, _, c := webFixture(t)
	c.signIn(t, "owner")
	if w := c.request("GET", "/account", nil, nil); w.Code != 200 || !strings.Contains(w.Body.String(), `value="UTC"`) {
		t.Fatal("missing default preference", w.Code)
	}
	if w := c.request("POST", "/account/timezone", url.Values{"timezone": {"Asia/Tokyo"}}, nil); w.Code != 403 {
		t.Fatal("missing CSRF accepted", w.Code)
	}
	if w := c.form(t, "/account/timezone", url.Values{"timezone": {"America/Los_Angeles"}, "user_id": {"2"}}, false); w.Code != 303 {
		t.Fatal(w.Code, w.Body.String())
	}
	if zone, err := s.Timezone(t.Context(), 1); err != nil || zone != "America/Los_Angeles" {
		t.Fatal(zone, err)
	}
	if zone, err := s.Timezone(t.Context(), 2); err != nil || zone != "UTC" {
		t.Fatal("changed another account", zone, err)
	}
	id, err := s.CreatePost(t.Context(), 1, "timezone fixture", testKey(), nil)
	if err != nil {
		t.Fatal(err)
	}
	stamp := time.Date(2026, 1, 1, 0, 15, 0, 0, time.UTC)
	if _, err = s.db.Exec("UPDATE posts SET created_at=? WHERE id=?", stamp.UnixMilli(), id); err != nil {
		t.Fatal(err)
	}
	for _, headers := range []map[string]string{nil, {"HX-Request": "true"}} {
		w := c.request("GET", fmt.Sprintf("/posts/%d", id), nil, headers)
		if w.Code != 200 || !strings.Contains(w.Body.String(), "Dec 31, 2025 · 16:15 PST") || !strings.Contains(w.Body.String(), `datetime="2026-01-01T00:15:00Z"`) {
			t.Fatal("local/UTC date disagreement", w.Code, w.Body.String())
		}
	}
	for _, values := range []url.Values{{"timezone": {"Local"}}, {"timezone": {"UTC", "Asia/Tokyo"}}, {"timezone": {"<script>bad</script>"}}} {
		w := c.form(t, "/account/timezone", values, false)
		want := 422
		if len(values["timezone"]) > 1 {
			want = 400
		}
		if w.Code != want || strings.Contains(w.Body.String(), "<script>bad</script>") {
			t.Fatal("invalid preference accepted or unescaped", w.Code)
		}
		if zone, _ := s.Timezone(t.Context(), 1); zone != "America/Los_Angeles" {
			t.Fatal("invalid save changed preference")
		}
	}
	if _, err = s.db.Exec("UPDATE users SET disabled=1 WHERE id=1"); err != nil {
		t.Fatal(err)
	}
	if s.SetTimezone(t.Context(), 1, "UTC") == nil {
		t.Fatal("disabled account changed preference")
	}
}
