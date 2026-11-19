package folkweave

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestPhotoViewerOrderAndPrivacy(t *testing.T) {
	s, _, c := webFixture(t)
	photos := []Photo{testPhoto(t), testPhoto(t), testPhoto(t)}
	for i := range photos {
		photos[i].Alt = fmt.Sprintf("Photo %d from the walk", i+1)
	}
	id, err := s.CreatePost(t.Context(), 1, "A walk", testKey(), photos)
	if err != nil {
		t.Fatal(err)
	}
	other, err := s.CreatePost(t.Context(), 1, "Another walk", testKey(), []Photo{testPhoto(t)})
	if err != nil {
		t.Fatal(err)
	}
	post, err := s.Post(t.Context(), 1, id)
	if err != nil {
		t.Fatal(err)
	}
	paths := make([]string, len(post.Photos))
	for i, photo := range post.Photos {
		paths[i] = photoViewURL(id, photo.ID)
	}
	if w := c.request("GET", paths[0], nil, nil); w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/login" {
		t.Fatalf("anonymous view: %d", w.Code)
	}
	c.signIn(t, "owner")
	for i, path := range paths {
		w := c.request("GET", path, nil, nil)
		body := w.Body.String()
		if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" ||
			!strings.Contains(body, fmt.Sprintf("Photo %d of 3", i+1)) ||
			!strings.Contains(body, `href="`+post.URL()+`">Back to post`) ||
			!strings.Contains(body, `<figcaption>`+photos[i].Alt+`</figcaption>`) {
			t.Fatalf("view of photo %d: %d", i+1, w.Code)
		}
		for _, nav := range []struct {
			offset int
			label  string
		}{{-1, "Previous photo"}, {1, "Next photo"}} {
			neighbor := i + nav.offset
			if neighbor < 0 || neighbor >= len(paths) {
				if strings.Contains(body, nav.label) {
					t.Fatalf("navigation wraps at photo %d", i+1)
				}
				continue
			}
			if !strings.Contains(body, `href="`+paths[neighbor]+`">`+nav.label) {
				t.Fatalf("wrong neighbor at photo %d", i+1)
			}
		}
	}
	if w := c.request("GET", photoViewURL(other, post.Photos[0].ID), nil, nil); w.Code != http.StatusNotFound {
		t.Fatalf("wrong-parent photo: %d", w.Code)
	}
	if w := c.request("GET", photoViewURL(id, "%27%3Cscript%3E"), nil, nil); w.Code != http.StatusNotFound {
		t.Fatalf("invalid photo ID: %d", w.Code)
	}
	if w := c.request("GET", post.URL(), nil, nil); !strings.Contains(w.Body.String(), `href="`+paths[0]+`"`) {
		t.Fatal("post photo does not link to the viewer")
	}
}

func TestSearchRoutesAndLiveContent(t *testing.T) {
	s, _, c := webFixture(t)
	member := testMember(t, s, "reader")
	photo := testPhoto(t)
	photo.Alt = "The Réunion group in front of an old oak"
	id, err := s.CreatePost(t.Context(), 1, strings.Repeat("A long letter. ", 20)+"Réunion <script> 100%", testKey(), []Photo{photo})
	if err != nil {
		t.Fatal(err)
	}
	reply, err := s.AddReply(t.Context(), member, id, "RÉUNION is next week", testKey())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateRepost(t.Context(), member, id, false); err != nil {
		t.Fatal(err)
	}
	path := "/search?q=" + url.QueryEscape("RÉUNION")
	if w := c.request("GET", path, nil, nil); w.Code != http.StatusSeeOther || strings.Contains(w.Body.String(), "Réunion") {
		t.Fatalf("anonymous search: %d", w.Code)
	}
	c.signIn(t, "reader")
	w := c.request("GET", path, nil, nil)
	body := w.Body.String()
	post, err := s.Post(t.Context(), member, id)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{post.URL() + `">Post by Owner`, fmt.Sprintf(`%s#reply-%d">Reply by Reader`, post.URL(), reply), photoViewURL(id, post.Photos[0].ID) + `">Photo description by Owner`, "Réunion &lt;script&gt; 100%"} {
		if w.Code != 200 || !strings.Contains(body, expected) {
			t.Fatalf("search missing %q (%d)", expected, w.Code)
		}
	}
	if strings.Count(body, "Post by Owner") != 1 || strings.Contains(body, "Réunion <script>") || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("search duplicated a repost, exposed markup, or allowed caching")
	}
	for _, bad := range []string{"?q=one&q=two", "?q=%FF", "?q=%ZZ", "?q=%00", "?q=" + strings.Repeat("x", 121), "?q=valid&before=invalid"} {
		if w := c.request("GET", "/search"+bad, nil, nil); w.Code != http.StatusBadRequest {
			t.Fatalf("invalid search %q returned %d", bad, w.Code)
		}
	}
	result, err := s.search(t.Context(), "%", "")
	if err != nil || len(result.Hits) != 1 || result.Hits[0].Kind() != "Post" {
		t.Fatalf("percent should be literal: %+v, %v", result, err)
	}
	if err := s.EditPost(t.Context(), 1, id, 1, "A corrected update"); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteContent(t.Context(), member, reply, true); err != nil {
		t.Fatal(err)
	}
	result, err = s.search(t.Context(), "réunion", "")
	if err != nil || len(result.Hits) != 1 || result.Hits[0].Kind() != "Photo description" {
		t.Fatalf("search kept stale text: %+v, %v", result, err)
	}
	if err := s.DeleteContent(t.Context(), 1, id, false); err != nil {
		t.Fatal(err)
	}
	result, err = s.search(t.Context(), "réunion", "")
	if err != nil || len(result.Hits) != 0 {
		t.Fatalf("deleted content appears in search: %+v, %v", result, err)
	}
}

func TestSearchPaginationWithTiedPostsAndReplies(t *testing.T) {
	s := testStore(t)
	for i := range PageSize + 3 {
		id, err := s.CreatePost(t.Context(), 1, fmt.Sprintf("Shared word in post %d", i), testKey(), nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.AddReply(t.Context(), 1, id, "Shared word in reply", testKey()); err != nil {
			t.Fatal(err)
		}
	}
	for _, statement := range []string{"UPDATE posts SET created_at=123", "UPDATE replies SET created_at=123"} {
		if _, err := s.db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	seen := make(map[string]bool)
	var before string
	for range 3 {
		page, err := s.search(t.Context(), "shared word", before)
		if err != nil || len(page.Hits) > PageSize {
			t.Fatalf("invalid page: %+v, %v", page, err)
		}
		for _, hit := range page.Hits {
			if seen[hit.URL()] {
				t.Fatalf("page repeated %s", hit.URL())
			}
			seen[hit.URL()] = true
		}
		if page.Next == "" {
			break
		}
		next, err := url.Parse(page.Next)
		if err != nil || next.Path != "/search" || next.Query().Get("q") != "shared word" {
			t.Fatalf("failed to keep search term: %s", page.Next)
		}
		before = next.Query().Get("before")
	}
	if want := 2 * (PageSize + 3); len(seen) != want {
		t.Fatalf("got %d matches, want %d", len(seen), want)
	}
}
