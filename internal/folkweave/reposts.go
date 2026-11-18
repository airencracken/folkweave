package folkweave

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/airencracken/comfylib/token"
)

type Repost struct {
	Kind     string          `json:"kind"`
	PostID   int64           `json:"post_id,omitempty"`
	ReplyID  int64           `json:"reply_id,omitempty"`
	Original *RepostOriginal `json:"original,omitempty"`
}

type RepostOriginal struct {
	AuthorID   int64   `json:"author_id"`
	AuthorName string  `json:"author_name"`
	Body       string  `json:"body"`
	Photos     []Photo `json:"photos,omitempty"`
}

func (r Repost) URL() string {
	path := fmt.Sprintf("/posts/%d", r.PostID)
	if r.ReplyID != 0 {
		return fmt.Sprintf("%s#reply-%d", path, r.ReplyID)
	}
	return path
}

func (s *Store) loadPostPhotos(ctx context.Context, p *Post) error {
	var err error
	switch {
	case p.Repost == nil:
		p.Photos, err = s.photos(ctx, p.ID)
	case p.Repost.Kind == "post" && p.Repost.Original != nil:
		p.Repost.Original.Photos, err = s.photos(ctx, p.Repost.PostID)
	}
	return err
}

// CreateRepost puts a reference on the timeline. Repeating the action opens
// the member's existing repost, leaving its date and discussion intact.
func (s *Store) CreateRepost(ctx context.Context, actor, source int64, reply bool) (int64, error) {
	kind, column := "post", "repost_post_id"
	lookup := "SELECT user_id FROM posts WHERE id=? AND repost_kind IS NULL"
	if reply {
		kind, column = "reply", "repost_reply_id"
		lookup = "SELECT user_id FROM replies WHERE id=?"
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if err := requireUser(ctx, tx, actor, false); err != nil {
		return 0, err
	}
	var author int64
	if err := tx.QueryRowContext(ctx, lookup, source).Scan(&author); err != nil {
		return 0, err
	}
	if author == actor {
		return 0, ErrForbidden
	}

	var id int64
	err = tx.QueryRowContext(ctx, "SELECT id FROM posts WHERE user_id=? AND "+column+"=?", actor, source).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	key, print := token.New()
	insert := "INSERT INTO posts(user_id,body,created_at,submission_key,fingerprint,repost_kind," + column + ") VALUES(?,'',?,?,?,?,?) RETURNING id"
	if err := tx.QueryRowContext(ctx, insert, actor, time.Now().UnixMilli(), key, print, kind, source).Scan(&id); err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

func (a *App) repost(reply bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		source, err := routeID(r)
		if err != nil {
			a.fail(w, r, err)
			return
		}
		id, err := a.store.CreateRepost(r.Context(), state(r).User.ID, source, reply)
		if err != nil {
			a.fail(w, r, err)
			return
		}
		a.redirect(w, r, Post{ID: id}.URL())
	}
}
