package folkweave

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadOnlyDemoStartupRollsBackOnError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	for _, tc := range []struct {
		name string
		ctx  context.Context
		base string
	}{
		{"fixture preparation", canceled, "http://example.test"},
		{"app configuration", t.Context(), "://invalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, cleanup, err := NewReadOnlyDemo(tc.ctx, Config{BaseURL: tc.base})
			if cleanup != nil {
				t.Cleanup(func() { _ = cleanup() })
			}
			if err == nil || app != nil || cleanup != nil {
				t.Fatal("invalid demo startup returned an application or cleanup handle")
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 0 {
				t.Fatalf("failed startup left sample data: %v, %v", entries, err)
			}
		})
	}
}

func TestReadOnlyDemoRouteAndStorageIsolation(t *testing.T) {
	app, cleanup, err := NewReadOnlyDemo(t.Context(), Config{BaseURL: "http://example.test"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cleanup() })
	request := func(method, path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "http://example.test"+path, nil)
		req.AddCookie(&http.Cookie{Name: app.cookieName("session"), Value: testKey()})
		w := httptest.NewRecorder()
		app.ServeHTTP(w, req)
		if w.Header().Get("Set-Cookie") != "" {
			t.Fatalf("sample issued a cookie for %s", path)
		}
		return w
	}
	feed, err := app.store.Posts(t.Context(), 0, "", false, 0)
	if err != nil || len(feed.Posts) == 0 || len(feed.Posts[0].Photos) == 0 {
		t.Fatalf("missing fictional photos: %v", err)
	}
	post := feed.Posts[0]
	photo := post.Photos[0]
	for path, text := range map[string]string{
		"/": "The Sunday Folks", "/search?q=woods": "Anyone fancy a walk",
		post.URL(): "Took the long way home", "/people": "Rowan", "/people/1": "Ari",
		photoViewURL(post.ID, photo.ID): "Photo 1 of 2",
	} {
		w := request("GET", path)
		body := w.Body.String()
		if w.Code != 200 || !strings.Contains(body, text) || !strings.Contains(body, "Read-only demo with fictional posts and photos.") ||
			strings.Contains(body, `method="post"`) || strings.Contains(body, "Your account") {
			t.Fatalf("bad sample page %s: %d", path, w.Code)
		}
	}
	for _, path := range []string{photo.URL(), photo.URL() + "?width=320", photo.URL() + "?width=640", photo.URL() + "?width=960"} {
		w := request("GET", path)
		if w.Code != 200 || w.Header().Get("Content-Type") != "image/jpeg" {
			t.Fatalf("sample photo %s: %d", path, w.Code)
		}
	}
	for _, path := range []string{"/login", "/join", "/saved", "/catch-up", "/replies", "/settings", "/account/export", "/api/v1/posts", "/posts/1/edit"} {
		if w := request("GET", path); w.Code != http.StatusNotFound {
			t.Fatalf("sample exposes %s: %d", path, w.Code)
		}
	}
	for _, path := range []string{"/posts", post.URL() + "/delete", post.URL() + "/save", post.URL() + "/follow", "/catch-up", "/replies/seen", "/login", "/join", "/logout"} {
		if w := request("POST", path); w.Code != http.StatusNotFound && w.Code != http.StatusMethodNotAllowed {
			t.Fatalf("sample allows POST %s: %d", path, w.Code)
		}
	}
	if _, err := app.store.db.Exec("DELETE FROM posts"); err == nil {
		t.Fatal("the sample store allowed a write")
	}
	directory := filepath.Dir(app.store.Path)
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(directory); !os.IsNotExist(err) {
		t.Fatalf("sample files survived cleanup: %v", err)
	}
}
