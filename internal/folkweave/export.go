package folkweave

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
)

type exportedReply struct {
	ID, PostID, CreatedAt int64
	Body                  string
}

type accountArchive struct {
	Version int             `json:"version"`
	User    User            `json:"user"`
	Posts   []Post          `json:"posts"`
	Replies []exportedReply `json:"replies"`
}

func (a *App) exportAccount(w http.ResponseWriter, r *http.Request) {
	select {
	case a.exports <- struct{}{}:
		defer func() { <-a.exports }()
	default:
		w.Header().Set("Retry-After", "10")
		a.problems.Write(w, r, http.StatusServiceUnavailable, "Another export is being prepared. Try again shortly.", nil)
		return
	}

	file, err := os.CreateTemp("", "folkweave-account-*.zip")
	if err != nil {
		a.exportError(w, r, err)
		return
	}
	defer os.Remove(file.Name())
	defer file.Close()

	if err := a.store.writeAccountArchive(r.Context(), file, *state(r).User); err != nil {
		a.exportError(w, r, err)
		return
	}
	size, err := file.Seek(0, io.SeekCurrent)
	if err == nil {
		_, err = file.Seek(0, io.SeekStart)
	}
	if err != nil {
		a.exportError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="folkweave-account.zip"`)
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	if r.Method == http.MethodHead {
		return
	}
	if n, err := io.Copy(w, file); err != nil || n != size {
		panic(http.ErrAbortHandler)
	}
}

func (a *App) exportError(w http.ResponseWriter, r *http.Request, err error) {
	a.problems.Write(w, r, http.StatusInternalServerError, "Your archive could not be prepared. Please try again.", err)
}

func (s *Store) writeAccountArchive(ctx context.Context, dst io.Writer, user User) error {
	data := accountArchive{Version: 1, User: user, Posts: []Post{}}
	for cursor := ""; ; {
		page, err := s.Posts(ctx, user.ID, cursor, false, user.ID)
		if err != nil {
			return err
		}
		data.Posts = append(data.Posts, page.Posts...)
		if page.Next == "" {
			break
		}
		cursor = page.Next
	}

	var err error
	data.Replies, err = s.exportReplies(ctx, user.ID)
	if err != nil {
		return err
	}
	archive := zip.NewWriter(dst)
	writer, err := archive.Create("account.json")
	if err != nil {
		return err
	}
	if err := json.NewEncoder(writer).Encode(data); err != nil {
		return err
	}
	for _, post := range data.Posts {
		for _, photo := range post.Photos {
			if err := s.writeExportedPhoto(ctx, archive, photo.ID); err != nil {
				return err
			}
		}
	}
	return archive.Close()
}

func (s *Store) exportReplies(ctx context.Context, userID int64) ([]exportedReply, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id,post_id,body,created_at FROM replies WHERE user_id=? ORDER BY id", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	replies := []exportedReply{}
	for rows.Next() {
		var reply exportedReply
		if err := rows.Scan(&reply.ID, &reply.PostID, &reply.Body, &reply.CreatedAt); err != nil {
			return nil, err
		}
		replies = append(replies, reply)
	}
	return replies, rows.Err()
}

func (s *Store) writeExportedPhoto(ctx context.Context, archive *zip.Writer, id string) error {
	photo, err := s.Photo(ctx, id)
	if err != nil {
		return fmt.Errorf("read export photo: %w", err)
	}
	writer, err := archive.Create("photos/" + photo.ID + ".jpg")
	if err != nil {
		return err
	}
	_, err = writer.Write(photo.Data)
	return err
}
