package folkweave

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func readingPost(t *testing.T, s *Store, author int64, body string) int64 {
	t.Helper()
	id, err := s.CreatePost(t.Context(), author, body, testKey(), nil)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func readingReply(t *testing.T, s *Store, author, post int64, body string) int64 {
	t.Helper()
	id, err := s.AddReply(t.Context(), author, post, body, testKey())
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestCatchUpUsesPrivateBookmarkAndFiniteSnapshot(t *testing.T) {
	s := testStore(t)
	reader := testMember(t, s, "reader")
	var expected []int64
	for i := range PageSize*2 + 3 {
		expected = append(expected, readingPost(t, s, 1, fmt.Sprintf("Update %d", i)))
	}
	first, err := s.catchUp(t.Context(), reader, 0, 0)
	if err != nil || len(first.Posts) != PageSize || first.MarkThrough != expected[PageSize-1] || first.Next == "" {
		t.Fatalf("bad first catch-up page: %d posts, mark %d, next %q, %v", len(first.Posts), first.MarkThrough, first.Next, err)
	}
	var total int
	if err := s.db.QueryRow("SELECT count(*) FROM reader_progress").Scan(&total); err != nil || total != 0 {
		t.Fatal("opening Catch up wrote a bookmark", total, err)
	}

	late := readingPost(t, s, 1, "Arrived while catching up")
	ids := collectCatchUp(t, s, reader, first)
	if !slices.Equal(ids, expected) || ids[len(ids)-1] == late {
		t.Fatalf("snapshot changed during pagination: got %v, want %v", ids, expected)
	}
	if err := s.markReadingProgress(t.Context(), reader, first.MarkThrough, 0, true); err != nil {
		t.Fatal(err)
	}
	next, err := s.catchUp(t.Context(), reader, 0, 0)
	if err != nil || len(next.Posts) == 0 || next.Posts[0].ID != expected[PageSize] {
		t.Fatal("bookmark skipped or repeated an update", next, err)
	}
	owner, err := s.catchUp(t.Context(), 1, 0, 0)
	if err != nil || len(owner.Posts) == 0 || owner.Posts[0].ID != expected[0] {
		t.Fatal("one reader moved another reader's bookmark", owner, err)
	}
	if err := s.markReadingProgress(t.Context(), reader, expected[len(expected)-1], 0, true); err != nil {
		t.Fatal(err)
	}
	if err := s.markReadingProgress(t.Context(), reader, first.MarkThrough, 0, true); err != nil {
		t.Fatal(err)
	}
	onlyLate, err := s.catchUp(t.Context(), reader, 0, 0)
	if err != nil || len(onlyLate.Posts) != 1 || onlyLate.Posts[0].ID != late {
		t.Fatal("an old form moved the bookmark backward or skipped later posts", onlyLate, err)
	}
	if err := s.DeleteContent(t.Context(), 1, late, false); err != nil {
		t.Fatal(err)
	}
	if err := s.markReadingProgress(t.Context(), reader, late, 0, true); err != nil {
		t.Fatal("deleting a shown post made the bookmark unusable", err)
	}
}

func collectCatchUp(t *testing.T, s *Store, reader int64, page catchUpPage) []int64 {
	t.Helper()
	var ids []int64
	for {
		for _, p := range page.Posts {
			ids = append(ids, p.ID)
		}
		if page.Next == "" {
			return ids
		}
		target, err := url.Parse(page.Next)
		if err != nil {
			t.Fatal(err)
		}
		after, err := readPosition(target.Query(), "after", true)
		if err != nil {
			t.Fatal(err)
		}
		through, err := readPosition(target.Query(), "through", true)
		if err != nil {
			t.Fatal(err)
		}
		page, err = s.catchUp(t.Context(), reader, after, through)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestReplyInboxOwnPostsFollowsAndSeenState(t *testing.T) {
	s := testStore(t)
	reader := testMember(t, s, "reader")
	friend := testMember(t, s, "friend")
	mine := readingPost(t, s, reader, "My update")
	theirs := readingPost(t, s, 1, "Their update")
	other := readingPost(t, s, friend, "Unrelated update")
	first := readingReply(t, s, 1, mine, "Reply to my post")
	earlier := readingReply(t, s, friend, theirs, "Before the follow")
	readingReply(t, s, reader, mine, "My own reply")
	readingReply(t, s, 1, other, "Unrelated conversation")
	if err := s.setDiscussionFollow(t.Context(), reader, theirs, true); err != nil {
		t.Fatal(err)
	}
	followed := readingReply(t, s, friend, theirs, "After the follow")
	if err := s.setDiscussionFollow(t.Context(), reader, theirs, true); err != nil {
		t.Fatal(err)
	}
	page := testInbox(t, s, reader, 0, followed, first)
	if page.MarkThrough != followed || !page.Replies[0].New || !page.Replies[1].New {
		t.Fatal("new replies should share the newest eligible id", page)
	}
	late := readingReply(t, s, friend, mine, "Another reply while the inbox was open")
	if err := s.markReadingProgress(t.Context(), reader, page.MarkThrough, 0, false); err != nil {
		t.Fatal(err)
	}
	if err := s.markReadingProgress(t.Context(), reader, first, 0, false); err != nil {
		t.Fatal(err)
	}
	page = testInbox(t, s, reader, 0, late, followed, first)
	if !page.Replies[0].New || page.Replies[1].New || page.Replies[2].New {
		t.Fatal("marking replies seen affected replies outside the rendered snapshot", page)
	}
	owner := testInbox(t, s, 1, 0, followed, earlier)
	if !owner.Replies[0].New {
		t.Fatal("one reader marked another reader's reply seen", owner)
	}
	if err := s.setDiscussionFollow(t.Context(), reader, theirs, false); err != nil {
		t.Fatal(err)
	}
	testInbox(t, s, reader, 0, late, first)
	if err := s.setDiscussionFollow(t.Context(), reader, theirs, true); err != nil {
		t.Fatal(err)
	}
	next := readingReply(t, s, friend, theirs, "Reply after refollow")
	testInbox(t, s, reader, 0, next, late, first)
	if err := s.DeleteContent(t.Context(), 1, theirs, false); err != nil {
		t.Fatal(err)
	}
	testInbox(t, s, reader, 0, late, first)
	if followed, err := s.followsDiscussion(t.Context(), reader, theirs); err != nil || followed {
		t.Fatal("deleting a discussion left an orphan follow", followed, err)
	}
}

func TestCatchUpBookmarkSubmissionKeepsItsSnapshot(t *testing.T) {
	s, _, owner := webFixture(t)
	for i := range PageSize + 1 {
		readingPost(t, s, 1, fmt.Sprintf("Update %d", i))
	}
	owner.signIn(t, "owner")
	if response := owner.form(t, "/catch-up", url.Values{"through": {"1"}, "snapshot": {"999"}}, false); response.Code != http.StatusUnprocessableEntity {
		t.Fatal("invalid submitted snapshot was accepted", response.Code)
	}
	page := owner.request(http.MethodGet, "/catch-up", nil, nil)
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "Update 0") ||
		!strings.Contains(page.Body.String(), fmt.Sprintf(`name="snapshot" value="%d"`, PageSize+1)) {
		t.Fatal("invalid mark advanced the bookmark or initial snapshot was missing", page.Code)
	}
	readingPost(t, s, 1, "Late arrival")
	through, snapshot := strconv.Itoa(PageSize), strconv.Itoa(PageSize+1)
	response := owner.form(t, "/catch-up", url.Values{"through": {through}, "snapshot": {snapshot}}, false)
	next := response.Header().Get("Location")
	if response.Code != http.StatusSeeOther || next != "/catch-up?after="+through+"&through="+snapshot {
		t.Fatal("marking a page lost its finite snapshot", response.Code, next)
	}
	page = owner.request(http.MethodGet, next, nil, nil)
	if page.Code != http.StatusOK || strings.Contains(page.Body.String(), "Late arrival") ||
		strings.Contains(page.Body.String(), "Continue catching up") {
		t.Fatal("new arrivals extended the in-progress catch-up batch", page.Code)
	}
}

func TestReplyInboxPagesRemainStableDuringNewReplies(t *testing.T) {
	s := testStore(t)
	friend := testMember(t, s, "friend")
	post := readingPost(t, s, 1, "Conversation")
	var expected []int64
	for i := range PageSize*2 + 1 {
		expected = append(expected, readingReply(t, s, friend, post, fmt.Sprintf("Reply %d", i)))
	}
	slices.Reverse(expected)
	before := int64(0)
	for i := 0; i < len(expected); i += PageSize {
		want := expected[i:min(i+PageSize, len(expected))]
		page := testInbox(t, s, 1, before, want...)
		if i == 0 {
			readingReply(t, s, friend, post, "A new reply while paging")
		}
		if i+PageSize < len(expected) {
			before = want[len(want)-1]
			if page.Next != fmt.Sprintf("/replies?before=%d", before) {
				t.Fatal("wrong older replies link", page.Next)
			}
		} else if page.Next != "" {
			t.Fatal("last page offers another page", page.Next)
		}
	}
}

func testInbox(t *testing.T, s *Store, reader, before int64, want ...int64) replyInboxPage {
	t.Helper()
	page, err := s.replyInbox(t.Context(), reader, before)
	var got []int64
	for _, reply := range page.Replies {
		got = append(got, reply.ID)
	}
	if err != nil || !slices.Equal(got, want) {
		t.Fatalf("inbox for %d: got %v, want %v, %v", reader, got, want, err)
	}
	return page
}

func TestReadingProgressRoutesAndAdversarialForms(t *testing.T) {
	s, app, owner := webFixture(t)
	reader := testMember(t, s, "reader")
	post := readingPost(t, s, 1, "Timeline entry")
	reply := readingReply(t, s, reader, post, "<script>new reply</script>")
	for _, path := range []string{"/catch-up", "/replies"} {
		response := owner.request(http.MethodGet, path, nil, nil)
		if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/login" {
			t.Fatalf("anonymous %s returned %d", path, response.Code)
		}
	}
	owner.signIn(t, "owner")
	for _, path := range []string{"/catch-up", "/replies"} {
		response := owner.request(http.MethodGet, path, nil, nil)
		if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" ||
			!strings.Contains(response.Body.String(), fmt.Sprintf(`href="%s" aria-current="page"`, path)) {
			t.Fatalf("signed-in route contract %s returned %d", path, response.Code)
		}
	}
	for _, path := range []string{
		"/catch-up?after=1", "/catch-up?through=1", "/catch-up?after=2&through=1", "/catch-up?after=1&through=99",
		"/catch-up?after=%2B1&through=1", "/catch-up?after=1&after=1&through=1",
		"/replies?before=9223372036854775808", "/replies?before=-1", "/replies?before=%zz",
	} {
		if response := owner.request(http.MethodGet, path, nil, nil); response.Code != http.StatusUnprocessableEntity {
			t.Errorf("invalid %s returned %d", path, response.Code)
		}
	}
	inbox := owner.request(http.MethodGet, "/replies", nil, nil).Body.String()
	if strings.Contains(inbox, "<script>new reply</script>") || !strings.Contains(inbox, "&lt;script&gt;new reply&lt;/script&gt;") ||
		!strings.Contains(inbox, fmt.Sprintf(`href="/posts/%d#reply-%d"`, post, reply)) {
		t.Fatal("inbox did not safely render and deep-link its reply")
	}
	for _, path := range []string{"/catch-up", "/replies/seen"} {
		for _, invalid := range []string{"0", "-1", "99", "<script>", "9223372036854775808"} {
			if response := owner.form(t, path, url.Values{"through": {invalid}}, false); response.Code != http.StatusUnprocessableEntity {
				t.Errorf("%s accepted invalid position %q: %d", path, invalid, response.Code)
			}
		}
		response := owner.request(http.MethodPost, path, url.Values{"through": {"1"}, "csrf": {"bad"}}, nil)
		if response.Code != http.StatusForbidden {
			t.Fatal("missing CSRF check", path, response.Code)
		}
	}
	if response := owner.request(http.MethodGet, "/replies/seen", nil, nil); response.Code != http.StatusMethodNotAllowed {
		t.Fatal("GET could mark replies seen", response.Code)
	}
	for path, id := range map[string]int64{"/catch-up": post, "/replies/seen": reply} {
		if response := owner.form(t, path, url.Values{"through": {strconv.FormatInt(id, 10)}}, false); response.Code != http.StatusSeeOther {
			t.Fatal(path, response.Code)
		}
	}
	member := &testClient{app: app, cookies: map[string]*http.Cookie{}}
	member.signIn(t, "reader")
	follow := fmt.Sprintf("/posts/%d/follow", post)
	if response := member.form(t, follow, url.Values{"follow": {"1"}}, false); response.Code != http.StatusSeeOther ||
		response.Header().Get("Location") != fmt.Sprintf("/posts/%d#replies", post) {
		t.Fatal("follow redirect contract", response.Code, response.Header().Get("Location"))
	}
	if response := member.request(http.MethodGet, fmt.Sprintf("/posts/%d", post), nil, nil); !strings.Contains(response.Body.String(), "Unfollow conversation") {
		t.Fatal("post did not show the reader's follow state")
	}
	if response := member.form(t, follow, url.Values{"follow": {"2"}}, false); response.Code != http.StatusUnprocessableEntity {
		t.Fatal("invalid follow accepted", response.Code)
	}
	if response := member.form(t, "/posts/999/follow", url.Values{"follow": {"1"}}, false); response.Code != http.StatusNotFound {
		t.Fatal("followed a missing discussion", response.Code)
	}

	response := owner.request(http.MethodGet, "/api/v1/posts", nil, nil)
	var api map[string]json.RawMessage
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &api) != nil || len(api) != 3 ||
		api["posts"] == nil || api["anchor"] == nil || api["next"] == nil ||
		strings.Contains(response.Body.String(), "caught_up") || strings.Contains(response.Body.String(), "seen_reply") ||
		strings.Contains(response.Body.String(), "following") {
		t.Fatal("reading tools changed the public posts API contract", response.Code, api)
	}
}

func TestReadingProgressMigrationRollsBackAndKeepsData(t *testing.T) {
	s := testStore(t)
	post := readingPost(t, s, 1, "Before the migration")
	if _, err := s.db.Exec(`
DROP TABLE reader_progress;
DROP TABLE discussion_follows;
DROP TABLE schema_version;
CREATE TABLE schema_version(version INTEGER NOT NULL CHECK(version=3));
INSERT INTO schema_version VALUES(3);
CREATE TABLE discussion_follows(conflict INTEGER);`); err != nil {
		t.Fatal(err)
	}
	if err := initializeSchema(t.Context(), s.db); err == nil {
		t.Fatal("conflicting schema migrated")
	}
	var version, progressTables int
	s.db.QueryRow("SELECT version FROM schema_version").Scan(&version)
	s.db.QueryRow("SELECT count(*) FROM sqlite_schema WHERE name='reader_progress'").Scan(&progressTables)
	if version != 3 || progressTables != 0 {
		t.Fatal("migration left partial schema", version, progressTables)
	}
	if _, err := s.db.Exec("DROP TABLE discussion_follows"); err != nil {
		t.Fatal(err)
	}
	if err := initializeSchema(t.Context(), s.db); err != nil {
		t.Fatal(err)
	}
	if err := s.Check(t.Context()); err != nil {
		t.Fatal(err)
	}
	if result, err := s.Post(t.Context(), 1, post); err != nil || result.Body != "Before the migration" {
		t.Fatal("migration lost content", result, err)
	}
	for _, invalid := range []string{
		"INSERT INTO reader_progress VALUES(999,0,0)",
		"INSERT INTO reader_progress VALUES(1,-1,0)",
		"INSERT INTO discussion_follows VALUES(1,999,0)",
		"INSERT INTO discussion_follows VALUES(1,1,-1)",
	} {
		if _, err := s.db.Exec(invalid); err == nil {
			t.Fatal("schema accepted invalid reading progress", invalid)
		}
	}
}

func TestReadingProgressConcurrentWritesAreMonotonic(t *testing.T) {
	s := testStore(t)
	member := testMember(t, s, "reader")
	var posts, replies []int64
	for i := range 12 {
		post := readingPost(t, s, 1, fmt.Sprintf("Post %d", i))
		posts = append(posts, post)
		replies = append(replies, readingReply(t, s, member, post, fmt.Sprintf("Reply %d", i)))
	}
	errs := make(chan error, len(posts)*2)
	var jobs sync.WaitGroup
	for i := range posts {
		jobs.Go(func() { errs <- s.markReadingProgress(t.Context(), member, posts[i], 0, true) })
		jobs.Go(func() { errs <- s.markReadingProgress(t.Context(), member, replies[i], 0, false) })
	}
	jobs.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var post, reply int64
	if err := s.db.QueryRow("SELECT caught_up_post_id,seen_reply_id FROM reader_progress WHERE user_id=?", member).Scan(&post, &reply); err != nil ||
		post != posts[len(posts)-1] || reply != replies[len(replies)-1] {
		t.Fatal("concurrent updates lost progress", post, reply, err)
	}

	if err := s.markReadingProgress(context.Background(), 999, posts[0], 0, true); !errors.Is(err, ErrForbidden) {
		t.Fatal("unknown account wrote a bookmark", err)
	}
}

func FuzzReadingPosition(f *testing.F) {
	for _, value := range []string{"1", "999", "0", "001", "9223372036854775808", "-1", "+10", " ", "\x00", "१२", ""} {
		f.Add(value)
	}
	f.Fuzz(func(t *testing.T, value string) {
		position, err := readPosition(url.Values{"through": {value}}, "through", true)
		if err == nil {
			if position < 1 || strings.TrimLeft(value, "0") != strconv.FormatInt(position, 10) {
				t.Fatalf("accepted a nonpositive or nonnumeric position %q", value)
			}
		}
	})
}
