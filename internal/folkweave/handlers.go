package folkweave

import (
	"archive/zip"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/airencracken/comfylib/token"
	"golang.org/x/crypto/bcrypt"
)

func (a *App) loginForm(w http.ResponseWriter, r *http.Request) {
	a.render(w, r, 200, Page{View: "login", Title: "Sign in"})
}
func (a *App) joinForm(w http.ResponseWriter, r *http.Request) {
	a.render(w, r, 200, Page{View: "join", Title: "Join the group", Invite: r.URL.Query().Get("invite")})
}
func (a *App) login(w http.ResponseWriter, r *http.Request) {
	if !a.authAllowed(w, r) {
		return
	}
	username := r.PostForm.Get("username")
	password := r.PostForm.Get("password")
	u, hash, err := a.store.Credentials(r.Context(), username)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		a.fail(w, r, err)
		return
	}
	if err != nil {
		hash = string(a.dummyHash)
	}
	match := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil || match != nil || len(password) > 72 || u.Disabled {
		a.render(w, r, 422, Page{View: "login", Title: "Sign in", Username: username, Error: "That username and password did not match."})
		return
	}
	a.signIn(w, r, u.ID)
}
func (a *App) join(w http.ResponseWriter, r *http.Request) {
	if !a.authAllowed(w, r) {
		return
	}
	username := strings.ToLower(strings.TrimSpace(r.PostForm.Get("username")))
	name := strings.TrimSpace(r.PostForm.Get("name"))
	password := r.PostForm.Get("password")
	secret := r.PostForm.Get("invite")
	p := Page{View: "join", Title: "Join the group", Username: username, Name: name, Invite: secret}
	err := ValidateCredentials(username, password)
	if err == nil && (name == "" || !validText(name, 80, false)) {
		err = errors.New("choose a display name up to 80 characters")
	}
	if err == nil && !validToken(secret) {
		err = ErrInvitation
	}
	if err != nil {
		p.Error = err.Error()
		a.render(w, r, 422, p)
		return
	}
	hash, err := HashPassword(password)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	id, err := a.store.Register(r.Context(), username, hash, name, secret)
	if err != nil {
		if errors.Is(err, ErrInvitation) || errors.Is(err, ErrUsername) {
			p.Error = err.Error()
			a.render(w, r, 422, p)
		} else {
			a.fail(w, r, err)
		}
		return
	}
	a.signIn(w, r, id)
}
func (a *App) signIn(w http.ResponseWriter, r *http.Request, id int64) {
	secret, err := a.store.NewSession(r.Context(), id)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	a.cookie(w, "session", secret, 7*86400)
	a.cookie(w, "form", "", -1)
	a.redirect(w, r, "/")
}
func (a *App) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(a.cookieName("session")); err == nil {
		if err = a.store.DeleteSession(r.Context(), token.Hash(cookie.Value)); err != nil {
			a.fail(w, r, err)
			return
		}
	}
	a.cookie(w, "session", "", -1)
	a.cookie(w, "form", "", -1)
	w.Header().Set("Clear-Site-Data", `"cache", "storage"`)
	a.redirect(w, r, "/login")
}

func (a *App) timeline(w http.ResponseWriter, r *http.Request) {
	saved := r.URL.Path == "/saved"
	feed, err := a.store.Posts(r.Context(), state(r).User.ID, r.URL.Query().Get("before"), saved, 0)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	p := Page{View: "timeline", Title: "Timeline", Feed: feed, Saved: saved}
	if saved {
		p.Title = "Saved posts"
	}
	if r.Header.Get("HX-Request") == "true" {
		a.partial(w, r, 200, "feed", p)
	} else {
		a.render(w, r, 200, p)
	}
}
func (a *App) updates(w http.ResponseWriter, r *http.Request) {
	after, err := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
	if err != nil || after < 0 {
		a.fail(w, r, ErrInvalid)
		return
	}
	var exists bool
	if err = a.store.db.QueryRowContext(r.Context(), "SELECT EXISTS(SELECT 1 FROM posts WHERE id>?)", after).Scan(&exists); err != nil {
		a.fail(w, r, err)
		return
	}
	a.partial(w, r, 200, "updates", Page{OOB: exists})
}
func (a *App) createPost(w http.ResponseWriter, r *http.Request) {
	body, key := r.PostForm.Get("body"), r.PostForm.Get("submission")
	var photos []Photo
	if r.MultipartForm != nil {
		files, alts := r.MultipartForm.File["photos"], r.PostForm["photo_alt"]
		if len(files) > MaxPhotos || len(alts) > MaxPhotos {
			a.postError(w, r, body, key, "Choose up to four photos.")
			return
		}
		for name := range r.MultipartForm.File {
			if name != "photos" {
				a.postError(w, r, body, key, "Unexpected attachment field.")
				return
			}
		}
		for i, file := range files {
			if file.Size > MaxPhotoBytes {
				a.postError(w, r, body, key, "Choose photos up to 5 MiB each.")
				return
			}
			reader, err := file.Open()
			if err != nil {
				a.fail(w, r, err)
				return
			}
			data, err := io.ReadAll(io.LimitReader(reader, MaxPhotoBytes+1))
			reader.Close()
			if err != nil {
				a.fail(w, r, err)
				return
			}
			alt := ""
			if i < len(alts) {
				alt = alts[i]
			}
			photo, err := NormalizePhoto(data, alt)
			if err != nil {
				a.postError(w, r, body, key, err.Error())
				return
			}
			photos = append(photos, photo)
		}
		for _, extra := range alts[min(len(files), len(alts)):] {
			if strings.TrimSpace(extra) != "" {
				a.postError(w, r, body, key, "Each description needs a matching photo.")
				return
			}
		}
	}
	_, err := a.store.CreatePost(r.Context(), state(r).User.ID, body, key, photos)
	if err != nil {
		if errors.Is(err, ErrInvalid) || errors.Is(err, ErrConflict) {
			message := "Write something or attach a photo. Posts can have up to 10,000 characters."
			if errors.Is(err, ErrConflict) {
				message = err.Error()
			}
			a.postError(w, r, body, key, message)
		} else {
			a.fail(w, r, err)
		}
		return
	}
	if r.Header.Get("HX-Request") != "true" {
		a.redirect(w, r, "/")
		return
	}
	feed, err := a.store.Posts(r.Context(), state(r).User.ID, "", false, 0)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	p := a.page(r, Page{Feed: feed, OOB: true})
	w.Header().Set("HX-Trigger", "folkweave:posted")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(201)
	a.templates.ExecuteTemplate(w, "composer", p)
	a.templates.ExecuteTemplate(w, "feed", p)
}
func (a *App) postError(w http.ResponseWriter, r *http.Request, body, key, message string) {
	if r.Header.Get("HX-Request") == "true" {
		a.formError(w, r, 422, message)
		return
	}
	feed, err := a.store.Posts(r.Context(), state(r).User.ID, "", false, 0)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	a.render(w, r, 422, Page{View: "timeline", Title: "Timeline", Body: body, Submission: key, Error: message + " Please reselect any photos.", Feed: feed})
}
func (a *App) post(w http.ResponseWriter, r *http.Request) {
	id, err := routeID(r)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	post, err := a.store.Post(r.Context(), state(r).User.ID, id)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	a.render(w, r, 200, Page{View: "post", Title: "Replies", Post: post})
}
func (a *App) reply(w http.ResponseWriter, r *http.Request) {
	id, err := routeID(r)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	body := r.PostForm.Get("body")
	_, err = a.store.AddReply(r.Context(), state(r).User.ID, id, body, r.PostForm.Get("submission"))
	if err != nil {
		if errors.Is(err, ErrInvalid) || errors.Is(err, ErrConflict) {
			if r.Header.Get("HX-Request") == "true" {
				a.formError(w, r, 422, "Write a reply up to 5,000 characters, then try again.")
			} else {
				p, e := a.store.Post(r.Context(), state(r).User.ID, id)
				if e != nil {
					a.fail(w, r, e)
					return
				}
				a.render(w, r, 422, Page{View: "post", Title: "Replies", Post: p, Body: body, Error: "Write a reply up to 5,000 characters.", Submission: r.PostForm.Get("submission")})
			}
		} else {
			a.fail(w, r, err)
		}
		return
	}
	if r.Header.Get("HX-Request") == "true" {
		post, err := a.store.Post(r.Context(), state(r).User.ID, id)
		if err != nil {
			a.fail(w, r, err)
			return
		}
		a.partial(w, r, 200, "discussion", Page{Post: post})
		return
	}
	a.redirect(w, r, fmt.Sprintf("/posts/%d#replies", id))
}
func (a *App) photo(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if len(id) != 32 || strings.Trim(id, "0123456789abcdef") != "" {
		http.NotFound(w, r)
		return
	}
	var data []byte
	var err error
	if values, ok := r.URL.Query()["width"]; ok {
		width, parseErr := strconv.Atoi(r.URL.Query().Get("width"))
		if len(values) != 1 || parseErr != nil || width != 320 && width != 640 && width != 960 {
			http.Error(w, "Choose a supported photo width", http.StatusBadRequest)
			return
		}
		select {
		case a.uploads <- struct{}{}:
			defer func() { <-a.uploads }()
		case <-r.Context().Done():
			return
		}
		data, err = a.store.PhotoPreview(r.Context(), id, width)
	} else {
		var photo Photo
		photo, err = a.store.Photo(r.Context(), id)
		data = photo.Data
	}
	if err != nil {
		a.fail(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Content-Disposition", `inline; filename="photo.jpg"`)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Write(data)
}
func (a *App) savePost(w http.ResponseWriter, r *http.Request) {
	id, err := routeID(r)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	value := r.PostForm.Get("save")
	if value != "0" && value != "1" {
		a.fail(w, r, ErrInvalid)
		return
	}
	if err = a.store.SavePost(r.Context(), state(r).User.ID, id, value == "1"); err != nil {
		a.fail(w, r, err)
		return
	}
	if r.Header.Get("HX-Request") == "true" {
		post, err := a.store.Post(r.Context(), state(r).User.ID, id)
		if err != nil {
			a.fail(w, r, err)
			return
		}
		a.partial(w, r, 200, "save-button", Page{Post: post})
		return
	}
	a.redirect(w, r, fmt.Sprintf("/posts/%d", id))
}
func (a *App) editForm(w http.ResponseWriter, r *http.Request) {
	id, err := routeID(r)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	post, err := a.store.Post(r.Context(), state(r).User.ID, id)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	if post.Author.ID != state(r).User.ID {
		a.fail(w, r, ErrForbidden)
		return
	}
	a.render(w, r, 200, Page{View: "edit", Title: "Edit your post", Post: post, Body: post.Body})
}
func (a *App) editPost(w http.ResponseWriter, r *http.Request) {
	id, err := routeID(r)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	revision, err := strconv.ParseInt(r.PostForm.Get("revision"), 10, 64)
	if err != nil {
		a.fail(w, r, ErrInvalid)
		return
	}
	err = a.store.EditPost(r.Context(), state(r).User.ID, id, revision, r.PostForm.Get("body"))
	if err != nil {
		a.fail(w, r, err)
		return
	}
	a.redirect(w, r, fmt.Sprintf("/posts/%d", id))
}
func (a *App) deletePost(w http.ResponseWriter, r *http.Request) {
	id, err := routeID(r)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	if r.PostForm.Get("confirmation") != "delete" {
		a.fail(w, r, ErrInvalid)
		return
	}
	if err = a.store.DeleteContent(r.Context(), state(r).User.ID, id, false); err != nil {
		a.fail(w, r, err)
		return
	}
	a.redirect(w, r, "/")
}
func (a *App) deleteReply(w http.ResponseWriter, r *http.Request) {
	id, err := routeID(r)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	if r.PostForm.Get("confirmation") != "delete" {
		a.fail(w, r, ErrInvalid)
		return
	}
	var post int64
	if err = a.store.db.QueryRowContext(r.Context(), "SELECT post_id FROM replies WHERE id=?", id).Scan(&post); err != nil {
		a.fail(w, r, err)
		return
	}
	if err = a.store.DeleteContent(r.Context(), state(r).User.ID, id, true); err != nil {
		a.fail(w, r, err)
		return
	}
	a.redirect(w, r, fmt.Sprintf("/posts/%d#replies", post))
}

func (a *App) people(w http.ResponseWriter, r *http.Request) {
	people, err := a.store.People(r.Context(), state(r).User.Owner())
	if err != nil {
		a.fail(w, r, err)
		return
	}
	a.render(w, r, 200, Page{View: "people", Title: "People", People: people})
}
func (a *App) person(w http.ResponseWriter, r *http.Request) {
	id, err := routeID(r)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	person, err := a.store.User(r.Context(), id)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	feed, err := a.store.Posts(r.Context(), state(r).User.ID, r.URL.Query().Get("before"), false, id)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	a.render(w, r, 200, Page{View: "person", Title: person.Name, Name: person.Name, Bio: person.Bio, Author: id, Feed: feed})
}
func (a *App) memberStatus(w http.ResponseWriter, r *http.Request) {
	id, err := routeID(r)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	value := r.PostForm.Get("disabled")
	if value != "0" && value != "1" {
		a.fail(w, r, ErrInvalid)
		return
	}
	if err = a.store.SetDisabled(r.Context(), state(r).User.ID, id, value == "1"); err != nil {
		a.fail(w, r, err)
		return
	}
	a.redirect(w, r, "/people")
}
func (a *App) account(w http.ResponseWriter, r *http.Request) {
	u := state(r).User
	a.render(w, r, 200, Page{View: "account", Title: "Your account", Name: u.Name, Bio: u.Bio})
}
func (a *App) profile(w http.ResponseWriter, r *http.Request) {
	name, bio := r.PostForm.Get("name"), r.PostForm.Get("bio")
	if err := a.store.Profile(r.Context(), state(r).User.ID, name, bio); err != nil {
		a.render(w, r, 422, Page{View: "account", Title: "Your account", Name: name, Bio: bio, Error: "Choose a name up to 80 characters and a bio up to 1,000 characters."})
		return
	}
	a.redirect(w, r, "/account")
}
func (a *App) invitations(w http.ResponseWriter, r *http.Request) {
	items, err := a.store.Invitations(r.Context())
	if err != nil {
		a.fail(w, r, err)
		return
	}
	a.render(w, r, 200, Page{View: "invitations", Title: "Invitations", Invitations: items})
}
func (a *App) invite(w http.ResponseWriter, r *http.Request) {
	uses, err := strconv.Atoi(r.PostForm.Get("uses"))
	if err != nil {
		a.fail(w, r, ErrInvalid)
		return
	}
	secret, err := a.store.NewInvitation(r.Context(), state(r).User.ID, uses)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	items, err := a.store.Invitations(r.Context())
	if err != nil {
		a.fail(w, r, err)
		return
	}
	a.render(w, r, 201, Page{View: "invitations", Title: "Invitations", Invitations: items, InviteURL: a.base.String() + "/join?invite=" + secret})
}
func (a *App) revokeInvite(w http.ResponseWriter, r *http.Request) {
	id, err := routeID(r)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	if err = a.store.RevokeInvitation(r.Context(), state(r).User.ID, id); err != nil {
		a.fail(w, r, err)
		return
	}
	a.redirect(w, r, "/invitations")
}
func (a *App) settings(w http.ResponseWriter, r *http.Request) {
	a.render(w, r, 200, Page{View: "settings", Title: "Settings", Backup: a.Backups.Status()})
}
func (a *App) configure(w http.ResponseWriter, r *http.Request) {
	if err := a.store.Configure(r.Context(), state(r).User.ID, r.PostForm.Get("title"), r.PostForm.Get("about")); err != nil {
		a.fail(w, r, err)
		return
	}
	a.redirect(w, r, "/settings")
}
func (a *App) apiPosts(w http.ResponseWriter, r *http.Request) {
	p, err := a.store.Posts(r.Context(), state(r).User.ID, r.URL.Query().Get("before"), false, 0)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	a.json(w, p)
}
func (a *App) apiPost(w http.ResponseWriter, r *http.Request) {
	id, err := routeID(r)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	post, err := a.store.Post(r.Context(), state(r).User.ID, id)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	a.json(w, post)
}

func (a *App) exportAccount(w http.ResponseWriter, r *http.Request) {
	user := *state(r).User
	posts := []Post{}
	after := ""
	for {
		page, err := a.store.Posts(r.Context(), user.ID, after, false, user.ID)
		if err != nil {
			a.fail(w, r, err)
			return
		}
		posts = append(posts, page.Posts...)
		if page.Next == "" {
			break
		}
		after = page.Next
	}
	rows, err := a.store.db.QueryContext(r.Context(), "SELECT id,post_id,body,created_at FROM replies WHERE user_id=? ORDER BY id", user.ID)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	type exportedReply struct {
		ID, PostID, CreatedAt int64
		Body                  string
	}
	replies := []exportedReply{}
	for rows.Next() {
		var v exportedReply
		if err = rows.Scan(&v.ID, &v.PostID, &v.Body, &v.CreatedAt); err != nil {
			rows.Close()
			a.fail(w, r, err)
			return
		}
		replies = append(replies, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		a.fail(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="folkweave-account.zip"`)
	archive := zip.NewWriter(w)
	defer archive.Close()
	writer, err := archive.Create("account.json")
	if err != nil {
		return
	}
	if err = json.NewEncoder(writer).Encode(struct {
		Version int             `json:"version"`
		User    User            `json:"user"`
		Posts   []Post          `json:"posts"`
		Replies []exportedReply `json:"replies"`
	}{1, user, posts, replies}); err != nil {
		return
	}
	for _, post := range posts {
		for _, photo := range post.Photos {
			v, err := a.store.Photo(r.Context(), photo.ID)
			if err != nil {
				return
			}
			writer, err := archive.Create("photos/" + v.ID + ".jpg")
			if err != nil {
				return
			}
			if _, err = writer.Write(v.Data); err != nil {
				return
			}
		}
	}
}
