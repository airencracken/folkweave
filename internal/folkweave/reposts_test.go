package folkweave

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestRepostRoutesAndAPI(t *testing.T) {
	s, _, client := webFixture(t)
	member := testMember(t, s, "member")
	photo := testPhoto(t)
	const originalText = "A quiet morning <script>"
	source, err := s.CreatePost(t.Context(), 1, originalText, testKey(), []Photo{photo})
	if err != nil {
		t.Fatal(err)
	}
	originalURL := Post{ID: source}.URL()
	reply, err := s.AddReply(t.Context(), 1, source, "A reply from the owner", testKey())
	if err != nil {
		t.Fatal(err)
	}
	replyRoute := fmt.Sprintf("/replies/%d/repost", reply)
	if w := client.form(t, originalURL+"/repost", url.Values{}, false); w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/login" {
		t.Fatalf("anonymous repost: %d %s", w.Code, w.Header().Get("Location"))
	}
	client.signIn(t, "member")
	if w := client.request("GET", originalURL+"/repost", nil, nil); w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET repost: %d", w.Code)
	}
	if w := client.request("POST", originalURL+"/repost", nil, nil); w.Code != http.StatusForbidden {
		t.Fatalf("accepted repost without CSRF: %d", w.Code)
	}
	if w := client.form(t, "/posts/01/repost", url.Values{}, false); w.Code != http.StatusNotFound {
		t.Fatalf("accepted malformed post ID: %d", w.Code)
	}
	if w := client.form(t, replyRoute, url.Values{"source": {"1", "2"}}, false); w.Code != http.StatusBadRequest {
		t.Fatalf("accepted repeated form field: %d", w.Code)
	}
	sourcePage := client.request("GET", originalURL, nil, nil).Body.String()
	for _, path := range []string{originalURL + "/repost", replyRoute} {
		if !strings.Contains(sourcePage, `action="`+path+`"`) {
			t.Fatalf("missing eligible repost form: %s", path)
		}
	}

	postRepost := submitRepost(t, client, originalURL+"/repost")
	replyRepost := submitRepost(t, client, replyRoute)
	if again := submitRepost(t, client, originalURL+"/repost"); again != postRepost {
		t.Fatalf("repeated request was redirected to %s, want %s", again, postRepost)
	}
	w := client.request("GET", "/api/v1"+postRepost, nil, nil)
	var post Post
	decoder := json.NewDecoder(w.Body)
	decoder.DisallowUnknownFields()
	if w.Code != http.StatusOK || decoder.Decode(&post) != nil || post.Author.ID != member || post.Body != "" || len(post.Photos) != 0 {
		t.Fatalf("unexpected repost contract: status %d, post %+v", w.Code, post)
	}
	if post.Repost == nil || post.Repost.Kind != "post" || post.Repost.PostID != source || post.Repost.ReplyID != 0 {
		t.Fatalf("wrong post reference: %+v", post.Repost)
	}
	if preview := post.Repost.Original; preview == nil || preview.AuthorID != 1 || preview.AuthorName != "Owner" || preview.Body != originalText || len(preview.Photos) != 1 || preview.Photos[0].Alt != photo.Alt {
		t.Fatalf("preview missing source content: %+v", preview)
	}
	for _, pagePath := range []string{"/", postRepost} {
		body := client.request("GET", pagePath, nil, nil).Body.String()
		if !strings.Contains(body, "reposted a post") || !strings.Contains(body, "A quiet morning &lt;script&gt;") || strings.Contains(body, "A quiet morning <script>") {
			t.Fatalf("timeline or permalink missing escaped preview at %s", pagePath)
		}
	}
	replyBody := client.request("GET", replyRepost, nil, nil).Body.String()
	if !strings.Contains(replyBody, `href="`+originalURL+fmt.Sprintf("#reply-%d", reply)+`">Original reply`) {
		t.Fatal("reposted reply did not link to its anchor")
	}
	if w := client.request("GET", postRepost+"/edit", nil, nil); w.Code != http.StatusForbidden {
		t.Fatalf("repost edit returned %d", w.Code)
	}
	if w := client.form(t, postRepost+"/repost", url.Values{}, false); w.Code != http.StatusNotFound {
		t.Fatalf("reposting a repost returned %d", w.Code)
	}

	archiveResponse := client.request("GET", "/account/export", nil, nil)
	archive, err := zip.NewReader(bytes.NewReader(archiveResponse.Body.Bytes()), int64(archiveResponse.Body.Len()))
	if err != nil || len(archive.File) != 1 {
		t.Fatalf("repost export included original photo files: %v", err)
	}
	file, err := archive.File[0].Open()
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var account accountArchive
	if err := json.NewDecoder(file).Decode(&account); err != nil {
		t.Fatal(err)
	}
	if len(account.Posts) != 2 {
		t.Fatalf("wrong export post count: %d", len(account.Posts))
	}
	for _, item := range account.Posts {
		if item.Repost == nil || item.Repost.PostID != source || item.Repost.Original != nil {
			t.Fatalf("export lost a reference or copied another author's content: %+v", item.Repost)
		}
	}
}

func submitRepost(t *testing.T, client *testClient, path string) string {
	t.Helper()
	w := client.form(t, path, url.Values{}, false)
	location := w.Header().Get("Location")
	if w.Code != http.StatusSeeOther || !strings.HasPrefix(location, "/posts/") {
		t.Fatalf("repost %s: %d, %s", path, w.Code, location)
	}
	return location
}

func TestRepostSourceLifecycle(t *testing.T) {
	s := testStore(t)
	member := testMember(t, s, "member")
	source, err := s.CreatePost(t.Context(), 1, "Before editing", testKey(), nil)
	if err != nil {
		t.Fatal(err)
	}
	reply, err := s.AddReply(t.Context(), 1, source, "Original reply", testKey())
	if err != nil {
		t.Fatal(err)
	}
	postShare, err := s.CreateRepost(t.Context(), member, source, false)
	if err != nil {
		t.Fatal(err)
	}
	replyShare, err := s.CreateRepost(t.Context(), member, reply, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddReply(t.Context(), member, postShare, "Discussion on the repost", testKey()); err != nil {
		t.Fatal(err)
	}
	if err := s.EditPost(t.Context(), 1, source, 1, "After editing"); err != nil {
		t.Fatal(err)
	}
	if post, err := s.Post(t.Context(), member, postShare); err != nil || post.Repost == nil || post.Repost.Original == nil || post.Repost.Original.Body != "After editing" {
		t.Fatalf("repost did not reflect the edit: %+v, %v", post.Repost, err)
	}
	if err := s.DeleteContent(t.Context(), 1, reply, true); err != nil {
		t.Fatal(err)
	}
	assertRemovedSource(t, s, member, replyShare, "reply")
	if err := s.DeleteContent(t.Context(), member, postShare, false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Post(t.Context(), 1, source); err != nil {
		t.Fatalf("removing a repost removed its source: %v", err)
	}
	postShare, err = s.CreateRepost(t.Context(), member, source, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddReply(t.Context(), member, postShare, "Still here after removal", testKey()); err != nil {
		t.Fatal(err)
	}
	reply, err = s.AddReply(t.Context(), 1, source, "Reply removed along with its post", testKey())
	if err != nil {
		t.Fatal(err)
	}
	replyShare, err = s.CreateRepost(t.Context(), member, reply, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteContent(t.Context(), 1, source, false); err != nil {
		t.Fatal(err)
	}
	assertRemovedSource(t, s, member, replyShare, "reply")
	post := assertRemovedSource(t, s, member, postShare, "post")
	if len(post.Replies) != 1 || post.Replies[0].Body != "Still here after removal" {
		t.Fatal("source deletion changed the repost's discussion")
	}
	if _, err := s.CreateRepost(t.Context(), member, source, false); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("reposted a deleted original: %v", err)
	}
}

func assertRemovedSource(t *testing.T, s *Store, member, id int64, kind string) Post {
	t.Helper()
	post, err := s.Post(t.Context(), member, id)
	if err != nil || post.Repost == nil || post.Repost.Kind != kind || post.Repost.Original != nil || post.Repost.PostID != 0 || post.Repost.ReplyID != 0 {
		t.Fatalf("removed %s still has a preview or link: %+v, %v", kind, post.Repost, err)
	}
	return post
}

func TestRepostsAreAtomicAndUnique(t *testing.T) {
	s := testStore(t)
	member := testMember(t, s, "member")
	source, err := s.CreatePost(t.Context(), 1, "Original", testKey(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateRepost(t.Context(), 1, source, false); !errors.Is(err, ErrForbidden) {
		t.Fatalf("self repost: %v", err)
	}
	if _, err := s.db.Exec(`CREATE TRIGGER fail_repost BEFORE INSERT ON posts WHEN NEW.repost_kind IS NOT NULL
		BEGIN SELECT RAISE(ABORT,'fixture'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateRepost(t.Context(), member, source, false); err == nil {
		t.Fatal("injected write unexpectedly succeeded")
	}
	var total int
	if err := s.db.QueryRow("SELECT count(*) FROM posts").Scan(&total); err != nil || total != 1 {
		t.Fatalf("failed write left a timeline entry: %d, %v", total, err)
	}
	if _, err := s.db.Exec("DROP TRIGGER fail_repost"); err != nil {
		t.Fatal(err)
	}
	type result struct {
		id  int64
		err error
	}
	const workers = 8
	results := make(chan result, workers)
	for range workers {
		go func() {
			id, err := s.CreateRepost(t.Context(), member, source, false)
			results <- result{id, err}
		}()
	}
	var id int64
	for range workers {
		next := <-results
		if next.err != nil || (id != 0 && next.id != id) {
			t.Fatalf("concurrent repost diverged: %+v, first id %d", next, id)
		}
		id = next.id
	}
	if err := s.db.QueryRow("SELECT count(*) FROM posts").Scan(&total); err != nil || total != 2 {
		t.Fatalf("concurrent requests left %d posts: %v", total, err)
	}
	for _, statement := range []string{
		"UPDATE posts SET repost_kind='post',repost_post_id=id WHERE id=?",
		"UPDATE posts SET repost_kind='reply' WHERE id=?",
		"UPDATE posts SET repost_kind='unknown' WHERE id=?",
		"UPDATE posts SET repost_post_id=1000 WHERE id=?",
	} {
		if _, err := s.db.Exec(statement, id); err == nil {
			t.Fatalf("schema accepted an invalid reference: %s", statement)
		}
	}
}
