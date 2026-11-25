package folkweave

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type inboxReply struct {
	ID, PostID, AuthorID int64
	Author, PostAuthor   string
	Excerpt              string
	Created              time.Time
	New                  bool
}

func (r inboxReply) URL() string {
	return fmt.Sprintf("/posts/%d#reply-%d", r.PostID, r.ID)
}

type replyInboxPage struct {
	Replies     []inboxReply
	Next        string
	MarkThrough int64
}

func (s *Store) replyInbox(ctx context.Context, user, before int64) (replyInboxPage, error) {
	var page replyInboxPage
	rows, err := s.db.QueryContext(ctx, `
WITH following(post_id,after_reply_id) AS (
 SELECT id,0 FROM posts WHERE user_id=?
 UNION ALL
 SELECT f.post_id,f.after_reply_id FROM discussion_follows f JOIN posts p ON p.id=f.post_id
 WHERE f.user_id=? AND p.user_id<>?
)
SELECT r.id,r.post_id,u.id,u.name,author.name,
 CASE WHEN length(r.body)>240 THEN substr(r.body,1,240)||'…' ELSE r.body END,
 r.created_at,r.id>coalesce((SELECT seen_reply_id FROM reader_progress WHERE user_id=?),0)
FROM following f JOIN replies r ON r.post_id=f.post_id
JOIN users u ON u.id=r.user_id
JOIN posts p ON p.id=r.post_id JOIN users author ON author.id=p.user_id
WHERE r.user_id<>? AND r.id>f.after_reply_id AND (?=0 OR r.id<?)
ORDER BY r.id DESC LIMIT ?`, user, user, user, user, user, before, before, PageSize+1)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		var reply inboxReply
		var timestamp int64
		if err := rows.Scan(&reply.ID, &reply.PostID, &reply.AuthorID, &reply.Author, &reply.PostAuthor, &reply.Excerpt, &timestamp, &reply.New); err != nil {
			return page, err
		}
		reply.Created = time.UnixMilli(timestamp)
		page.Replies = append(page.Replies, reply)
	}
	if len(page.Replies) > PageSize {
		page.Replies = page.Replies[:PageSize]
		page.Next = fmt.Sprintf("/replies?before=%d", page.Replies[len(page.Replies)-1].ID)
	}
	if before == 0 && len(page.Replies) > 0 && page.Replies[0].New {
		page.MarkThrough = page.Replies[0].ID
	}
	return page, rows.Err()
}

func (s *Store) followsDiscussion(ctx context.Context, user, post int64) (bool, error) {
	var follows bool
	err := s.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM discussion_follows WHERE user_id=? AND post_id=?)",
		user, post).Scan(&follows)
	return follows, err
}

func (s *Store) setDiscussionFollow(ctx context.Context, user, post int64, follow bool) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := requireUser(ctx, tx, user, false); err != nil {
		return err
	}
	var author int64
	if err := tx.QueryRowContext(ctx, "SELECT user_id FROM posts WHERE id=?", post).Scan(&author); err != nil {
		return err
	}
	if author == user {
		return ErrInvalid
	}
	if follow {
		_, err = tx.ExecContext(ctx, "INSERT INTO discussion_follows VALUES(?,?,"+sequenceValue+") ON CONFLICT(user_id,post_id) DO NOTHING",
			user, post, "replies")
	} else {
		_, err = tx.ExecContext(ctx, "DELETE FROM discussion_follows WHERE user_id=? AND post_id=?", user, post)
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (a *App) replyInbox(w http.ResponseWriter, r *http.Request) {
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		a.fail(w, r, ErrInvalid)
		return
	}
	before, err := readPosition(values, "before", false)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	page, err := a.store.replyInbox(r.Context(), state(r).viewerID(), before)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	a.render(w, r, http.StatusOK, Page{View: "reply-inbox", Title: "Replies", Inbox: page})
}

func (a *App) followDiscussion(w http.ResponseWriter, r *http.Request) {
	post, err := routeID(r)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	values := r.PostForm["follow"]
	if len(values) != 1 || (values[0] != "1" && values[0] != "0") {
		a.fail(w, r, ErrInvalid)
		return
	}
	if err := a.store.setDiscussionFollow(r.Context(), state(r).viewerID(), post, values[0] == "1"); err != nil {
		a.fail(w, r, err)
		return
	}
	a.redirect(w, r, fmt.Sprintf("/posts/%d#replies", post))
}

func (a *App) renderDiscussion(w http.ResponseWriter, r *http.Request, status int, page Page) {
	if user := state(r).User; user != nil && user.ID != page.Post.Author.ID {
		following, err := a.store.followsDiscussion(r.Context(), user.ID, page.Post.ID)
		if err != nil {
			a.fail(w, r, err)
			return
		}
		page.FollowingDiscussion = following
	}
	a.render(w, r, status, page)
}
