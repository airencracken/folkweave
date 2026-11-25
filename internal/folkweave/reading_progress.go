package folkweave

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type catchUpPage struct {
	Posts       []Post
	Next        string
	MarkThrough int64
	Snapshot    int64
}

const sequenceValue = "(SELECT coalesce((SELECT seq FROM sqlite_sequence WHERE name=?),0))"

func (s *Store) catchUp(ctx context.Context, user, after, through int64) (catchUpPage, error) {
	var page catchUpPage
	var bookmark, latest int64
	err := s.db.QueryRowContext(ctx, `
SELECT coalesce((SELECT caught_up_post_id FROM reader_progress WHERE user_id=?),0), `+sequenceValue,
		user, "posts").Scan(&bookmark, &latest)
	if err != nil {
		return page, err
	}
	if through == 0 {
		through = latest
	}
	if through > latest {
		return page, ErrInvalid
	}
	posts, more, err := s.loadPostPage(ctx, postSelect+" WHERE p.id>? AND p.id<=? ORDER BY p.id LIMIT ?",
		user, max(bookmark, after), through, PageSize+1)
	if err != nil {
		return page, err
	}
	page.Posts = posts
	if len(posts) > 0 {
		page.MarkThrough = through
	}
	if more {
		page.MarkThrough = posts[len(posts)-1].ID
		page.Snapshot = through
		page.Next = catchUpURL(page.MarkThrough, through)
	}
	return page, nil
}

func catchUpURL(after, through int64) string {
	return fmt.Sprintf("/catch-up?after=%d&through=%d", after, through)
}

func (s *Store) markReadingProgress(ctx context.Context, user, through, snapshot int64, posts bool) error {
	if through < 1 || (snapshot != 0 && (!posts || snapshot <= through)) {
		return ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := requireUser(ctx, tx, user, false); err != nil {
		return err
	}

	sequence := "replies"
	var postID, replyID int64
	if posts {
		sequence, postID = "posts", through
	} else {
		replyID = through
	}
	var latest int64
	if err := tx.QueryRowContext(ctx, "SELECT "+sequenceValue, sequence).Scan(&latest); err != nil {
		return err
	}
	if max(through, snapshot) > latest {
		return ErrInvalid
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO reader_progress(user_id,caught_up_post_id,seen_reply_id) VALUES(?,?,?)
ON CONFLICT(user_id) DO UPDATE SET
 caught_up_post_id=max(reader_progress.caught_up_post_id,excluded.caught_up_post_id),
 seen_reply_id=max(reader_progress.seen_reply_id,excluded.seen_reply_id)`, user, postID, replyID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func readPosition(values url.Values, name string, required bool) (int64, error) {
	input, present := values[name]
	if !present && !required {
		return 0, nil
	}
	if len(input) != 1 || len(input[0]) > 19 || strings.IndexFunc(input[0], func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
		return 0, ErrInvalid
	}
	id, err := strconv.ParseInt(input[0], 10, 64)
	if err != nil || id < 1 {
		return 0, ErrInvalid
	}
	return id, nil
}

func (a *App) catchUp(w http.ResponseWriter, r *http.Request) {
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		a.fail(w, r, ErrInvalid)
		return
	}
	after, err := readPosition(values, "after", false)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	through, err := readPosition(values, "through", after > 0)
	if err != nil || after > through || (through > 0 && after == 0) {
		a.fail(w, r, ErrInvalid)
		return
	}
	page, err := a.store.catchUp(r.Context(), state(r).viewerID(), after, through)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	a.render(w, r, http.StatusOK, Page{View: "catch-up", Title: "Catch up", CatchUp: page})
}

func (a *App) markReadingProgress(w http.ResponseWriter, r *http.Request) {
	through, err := readPosition(r.PostForm, "through", true)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	posts := r.URL.Path == "/catch-up"
	snapshot, err := readPosition(r.PostForm, "snapshot", false)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	err = a.store.markReadingProgress(r.Context(), state(r).viewerID(), through, snapshot, posts)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	if posts {
		target := "/catch-up"
		if snapshot > 0 {
			target = catchUpURL(through, snapshot)
		}
		a.redirect(w, r, target)
		return
	}
	a.redirect(w, r, "/replies")
}
