package folkweave

import (
	"database/sql"
	"fmt"
	"net/http"
)

type photoGallery struct {
	PostID int64
	Photos []Photo
}

type photoView struct {
	Photo    Photo
	PostURL  string
	Previous string
	Next     string
	Number   int
	Total    int
}

func photoViewURL(postID int64, photoID string) string {
	return fmt.Sprintf("/posts/%d/photos/%s", postID, photoID)
}

func (g photoGallery) View(id string) (photoView, error) {
	for i, photo := range g.Photos {
		if photo.ID != id {
			continue
		}
		view := photoView{Photo: photo, PostURL: Post{ID: g.PostID}.URL(), Number: i + 1, Total: len(g.Photos)}
		if i > 0 {
			view.Previous = photoViewURL(g.PostID, g.Photos[i-1].ID)
		}
		if i+1 < len(g.Photos) {
			view.Next = photoViewURL(g.PostID, g.Photos[i+1].ID)
		}
		return view, nil
	}
	return photoView{}, sql.ErrNoRows
}

func (a *App) viewPhoto(w http.ResponseWriter, r *http.Request) {
	id, err := routeID(r)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	photos, err := a.store.photos(r.Context(), id)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	view, err := (photoGallery{PostID: id, Photos: photos}).View(r.PathValue("photo"))
	if err != nil {
		a.fail(w, r, err)
		return
	}
	a.render(w, r, http.StatusOK, Page{View: "photo", Title: "Photo", Photo: view})
}
