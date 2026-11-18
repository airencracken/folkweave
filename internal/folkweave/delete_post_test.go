package folkweave

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestAuthorCanDeletePostFromTimeline(t *testing.T) {
	s, app, author := webFixture(t)
	member := testMember(t, s, "member")
	other := testMember(t, s, "other")
	post, err := s.CreatePost(t.Context(), member, "A post to delete", testKey(), []Photo{testPhoto(t)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddReply(t.Context(), other, post, "An attached reply", testKey()); err != nil {
		t.Fatal(err)
	}
	if err := s.SavePost(t.Context(), other, post, true); err != nil {
		t.Fatal(err)
	}
	item, err := s.Post(t.Context(), member, post)
	if err != nil {
		t.Fatal(err)
	}
	photoURL := item.Photos[0].URL()
	if _, err := s.PhotoPreview(t.Context(), item.Photos[0].ID, 320); err != nil {
		t.Fatal(err)
	}
	author.signIn(t, "member")
	deleteURL := item.URL() + "/delete"
	for _, page := range []string{"/", item.URL()} {
		html := author.request(http.MethodGet, page, nil, nil).Body.String()
		if !strings.Contains(html, "<summary>Delete post</summary>") || strings.Count(html, `action="`+deleteURL+`"`) != 1 {
			t.Fatalf("%s is missing a single delete option", page)
		}
	}
	reader := &testClient{app: app, cookies: map[string]*http.Cookie{}}
	reader.signIn(t, "other")
	if html := reader.request(http.MethodGet, "/", nil, nil).Body.String(); strings.Contains(html, deleteURL) {
		t.Fatal("showed another member a delete control")
	}
	if response := reader.form(t, deleteURL, url.Values{"confirmation": {"delete"}}, false); response.Code != http.StatusForbidden {
		t.Fatalf("another member could delete the post: %d", response.Code)
	}
	if response := author.request(http.MethodGet, deleteURL, nil, nil); response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET deletion returned %d", response.Code)
	}
	if response := author.form(t, deleteURL, url.Values{}, false); response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("unconfirmed deletion returned %d", response.Code)
	}
	response := author.form(t, deleteURL, url.Values{"confirmation": {"delete"}}, false)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/" {
		t.Fatalf("author deletion returned %d, %s", response.Code, response.Header().Get("Location"))
	}
	for _, path := range []string{item.URL(), "/api/v1" + item.URL(), photoURL, photoURL + "?width=320"} {
		if response := reader.request(http.MethodGet, path, nil, nil); response.Code != http.StatusNotFound {
			t.Fatalf("deleted content remains available at %s: %d", path, response.Code)
		}
	}
	for _, table := range []string{"posts", "photos", "photo_previews", "replies", "saved"} {
		var remaining int
		if err := s.db.QueryRow("SELECT count(*) FROM " + table).Scan(&remaining); err != nil || remaining != 0 {
			t.Fatalf("deletion left %d rows in %s: %v", remaining, table, err)
		}
	}
}
