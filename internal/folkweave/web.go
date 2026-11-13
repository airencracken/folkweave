package folkweave

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/airencracken/comfylib/admin"
	"github.com/airencracken/comfylib/displaytime"
	"github.com/airencracken/comfylib/ratelimit"
	"github.com/airencracken/comfylib/suite"
	"html/template"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/airencracken/comfylib/autobackup"
	"github.com/airencracken/comfylib/clientip"
	"github.com/airencracken/comfylib/token"
	"github.com/airencracken/comfylib/ui"
	"golang.org/x/crypto/bcrypt"
)

//go:embed templates/*.html static/*.css static/app.js static/theme.js static/*.svg static/loom.webp static/favicon-32.png static/*.min.js static/*.LICENSE static/vendor.json
var assets embed.FS

type Config struct {
	Version        string
	BaseURL        string
	TrustedProxies string
}
type App struct {
	store     *Store
	config    Config
	base      *url.URL
	handler   http.Handler
	templates *displaytime.Templates
	dummyHash []byte
	resolver  clientip.Resolver
	limiter   rateLimiter
	uploads   chan struct{}
	Backups   *autobackup.Runner
}
type requestState struct {
	User *User
	CSRF string
}
type contextKey struct{}

func state(r *http.Request) requestState {
	v, _ := r.Context().Value(contextKey{}).(requestState)
	return v
}

type Page struct {
	TimezoneControl                                                           displaytime.Preference
	TimezoneInput                                                             *string
	TimezoneError, TimezoneNotice                                             string
	Overview                                                                  admin.View
	View, Title, Error, Body, Username, Name, Bio, Invite, Submission, Notice string
	Settings                                                                  Settings
	User                                                                      *User
	CSRF                                                                      string
	Feed                                                                      PostPage
	Post                                                                      Post
	People                                                                    []User
	Invitations                                                               []Invitation
	Saved                                                                     bool
	OOB                                                                       bool
	Author                                                                    int64
	InviteURL                                                                 string
	Backup                                                                    autobackup.Status
}

func NewApp(store *Store, c Config) (*App, error) {
	suiteFuncs, suiteErr := suite.TemplateFuncs("FOLKWEAVE_")
	if suiteErr != nil {
		return nil, suiteErr
	}
	u, err := url.Parse(c.BaseURL)
	if err != nil || u == nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, errors.New("base URL must be a complete HTTP or HTTPS origin")
	}
	u.Path = ""
	trusted, err := clientip.ParseTrusted(c.TrustedProxies, "FOLKWEAVE_TRUSTED_PROXIES")
	if err != nil {
		return nil, err
	}
	dummy, err := bcrypt.GenerateFromPassword([]byte("unused-account-password"), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	functions := template.FuncMap{"timezonePreference": displaytime.RenderPreference, "adminSummary": admin.Render, "linkify": linkify, "canEdit": func(u *User, p Post) bool { return u != nil && u.ID == p.Author.ID }, "canDelete": func(u *User, id int64) bool { return u != nil && (u.Owner() || u.ID == id) }}
	functions["postContext"] = func(root Page, post Post) Page { root.Post = post; return root }
	t, err := template.New("pages").Funcs(suiteFuncs).Funcs(functions).Funcs(displaytime.New("UTC").Funcs()).ParseFS(assets, "templates/*.html")
	if err != nil {
		return nil, err
	}
	bound, err := displaytime.NewTemplates(t, nil)
	if err != nil {
		return nil, err
	}
	a := &App{store: store, config: c, base: u, templates: bound, dummyHash: dummy, resolver: clientip.Resolver{Trusted: trusted}, uploads: make(chan struct{}, 2)}
	mux := http.NewServeMux()
	static, _ := fs.Sub(assets, "static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(static))))
	mux.Handle("GET /static/comfyware/", http.StripPrefix("/static/comfyware/", http.FileServer(http.FS(ui.Assets()))))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintln(w, "ok")
	})
	mux.HandleFunc("GET /login", a.loginForm)
	mux.HandleFunc("POST /login", a.login)
	mux.HandleFunc("GET /join", a.joinForm)
	mux.HandleFunc("POST /join", a.join)
	mux.HandleFunc("POST /logout", a.private(a.logout))
	mux.HandleFunc("GET /{$}", a.private(a.timeline))
	mux.HandleFunc("GET /saved", a.private(a.timeline))
	mux.HandleFunc("GET /updates", a.private(a.updates))
	mux.HandleFunc("POST /posts", a.private(a.createPost))
	mux.HandleFunc("GET /posts/{id}", a.private(a.post))
	mux.HandleFunc("GET /posts/{id}/edit", a.private(a.editForm))
	mux.HandleFunc("POST /posts/{id}/edit", a.private(a.editPost))
	mux.HandleFunc("POST /posts/{id}/delete", a.private(a.deletePost))
	mux.HandleFunc("POST /posts/{id}/save", a.private(a.savePost))
	mux.HandleFunc("POST /posts/{id}/replies", a.private(a.reply))
	mux.HandleFunc("POST /replies/{id}/delete", a.private(a.deleteReply))
	mux.HandleFunc("GET /photos/{id}", a.private(a.photo))
	mux.HandleFunc("GET /people", a.private(a.people))
	mux.HandleFunc("GET /people/{id}", a.private(a.person))
	mux.HandleFunc("POST /people/{id}/status", a.owner(a.memberStatus))
	mux.HandleFunc("GET /account", a.private(a.account))
	mux.HandleFunc("POST /account", a.private(a.profile))
	mux.HandleFunc("POST /account/timezone", a.private(a.saveTimezone))
	mux.HandleFunc("GET /account/export", a.private(a.exportAccount))
	mux.HandleFunc("GET /invitations", a.owner(a.invitations))
	mux.HandleFunc("POST /invitations", a.owner(a.invite))
	mux.HandleFunc("POST /invitations/{id}/revoke", a.owner(a.revokeInvite))
	mux.HandleFunc("GET /admin/metrics", a.owner(a.metrics))
	mux.HandleFunc("GET /settings", a.owner(a.settings))
	mux.HandleFunc("POST /settings", a.owner(a.configure))
	mux.HandleFunc("GET /api/v1/posts", a.private(a.apiPosts))
	mux.HandleFunc("GET /api/v1/posts/{id}", a.private(a.apiPost))
	mux.HandleFunc("GET /api/v1/me", a.private(func(w http.ResponseWriter, r *http.Request) { a.json(w, state(r).User) }))
	if err = a.registerSharedSignIn(mux); err != nil {
		return nil, err
	}
	a.handler = http.NewCrossOriginProtection().Handler(a.middleware(mux))
	return a, nil
}
func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) { a.handler.ServeHTTP(w, r) }
func (a *App) cookieName(name string) string {
	if a.base.Scheme == "https" {
		return "__Host-folkweave_" + name
	}
	return "folkweave_" + name
}
func (a *App) cookie(w http.ResponseWriter, name, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{Name: a.cookieName(name), Value: value, Path: "/", MaxAge: maxAge, HttpOnly: true, Secure: a.base.Scheme == "https", SameSite: http.SameSiteLaxMode})
}

func (a *App) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' blob:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Robots-Tag", "noindex, nofollow")
		if r.URL.Path == "/healthz" || strings.HasPrefix(r.URL.Path, "/static/") {
			next.ServeHTTP(w, r)
			return
		}
		if !strings.EqualFold(r.Host, a.base.Host) {
			http.Error(w, "Unknown host", 400)
			return
		}
		st := requestState{}
		basis := ""
		if c, err := r.Cookie(a.cookieName("session")); err == nil && validToken(c.Value) {
			u, err := a.store.Session(r.Context(), token.Hash(c.Value))
			if err == nil {
				u.Timezone, err = a.store.Timezone(r.Context(), u.ID)
				if err != nil {
					a.fail(w, r, err)
					return
				}
				st.User = &u
				basis = c.Value
			} else if !errors.Is(err, sql.ErrNoRows) {
				a.fail(w, r, err)
				return
			}
		}
		if basis == "" {
			if c, err := r.Cookie(a.cookieName("form")); err == nil && validToken(c.Value) {
				basis = c.Value
			} else {
				basis, _ = token.New()
				a.cookie(w, "form", basis, 86400)
			}
		}
		st.CSRF = token.SessionCSRF(basis, "folkweave-csrf-v1")
		r = r.WithContext(context.WithValue(r.Context(), contextKey{}, st))
		if r.Method == "POST" {
			limit := int64(256 << 10)
			multipart := strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data")
			if multipart && r.URL.Path == "/posts" && st.User != nil {
				limit = MaxPhotos*MaxPhotoBytes + (1 << 20)
				select {
				case a.uploads <- struct{}{}:
					defer func() { <-a.uploads }()
				case <-r.Context().Done():
					return
				}
			}
			r.Body = http.MaxBytesReader(w, r.Body, limit)
			var err error
			if multipart {
				err = r.ParseMultipartForm(1 << 20)
				if r.MultipartForm != nil {
					defer r.MultipartForm.RemoveAll()
				}
			} else {
				err = r.ParseForm()
			}
			if err != nil {
				a.formError(w, r, 413, "That submission is too large or could not be read.")
				return
			}
			if len(r.PostForm["csrf"]) != 1 || !token.Equal(r.PostForm.Get("csrf"), st.CSRF) {
				a.formError(w, r, 403, "This form has expired. Reload the page and try again.")
				return
			}
			for key, values := range r.PostForm {
				if key != "photo_alt" && len(values) != 1 {
					slog.Warn("rejected form with a repeated field", "path", r.URL.Path, "field", logValue(key))
					a.formError(w, r, 400, malformedSubmission)
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}
func (a *App) private(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if state(r).User == nil {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				http.Error(w, "Sign in required", 401)
				return
			}
			if strings.HasPrefix(r.URL.Path, "/photos/") {
				http.NotFound(w, r)
				return
			}
			a.redirect(w, r, "/login")
			return
		}
		next(w, r)
	}
}
func (a *App) owner(next http.HandlerFunc) http.HandlerFunc {
	return a.private(func(w http.ResponseWriter, r *http.Request) {
		if !state(r).User.Owner() {
			a.fail(w, r, ErrForbidden)
			return
		}
		next(w, r)
	})
}
func (a *App) redirect(w http.ResponseWriter, r *http.Request, target string) {
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", target)
		w.WriteHeader(200)
		return
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}
func (a *App) page(r *http.Request, p Page) Page {
	if state(r).User != nil && state(r).User.Owner() && (p.View == "settings" || p.View == "metrics") {
		p.Overview = admin.WithOperations(p.Overview, a.config.Version, ownerBackupStatus(a.Backups.Status()), "FOLKWEAVE_")
	}
	p.User = state(r).User
	p.Overview.Timezone = preferredTimezone(r)
	p.TimezoneControl = timezoneControl(r, p.TimezoneInput, p.TimezoneError, p.TimezoneNotice)
	p.CSRF = state(r).CSRF
	if p.Settings.Title == "" {
		p.Settings, _ = a.store.Settings(r.Context())
	}
	if p.Submission == "" {
		p.Submission, _ = token.New()
	}
	return p
}
func (a *App) render(w http.ResponseWriter, r *http.Request, status int, p Page) {
	p = a.page(r, p)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := a.templates.ExecuteTemplate(w, preferredTimezone(r), "layout", p); err != nil {
		slog.Error("render page", "error", err)
	}
}
func (a *App) partial(w http.ResponseWriter, r *http.Request, status int, name string, p Page) {
	p = a.page(r, p)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := a.templates.ExecuteTemplate(w, preferredTimezone(r), name, p); err != nil {
		slog.Error("render fragment", "error", err)
	}
}
func (a *App) formError(w http.ResponseWriter, r *http.Request, status int, message string) {
	if r.Header.Get("HX-Request") == "true" {
		target := "#composer-error"
		if strings.Contains(r.URL.Path, "/replies") {
			target = "#reply-error"
		}
		w.Header().Set("HX-Retarget", target)
		w.Header().Set("HX-Reswap", "innerHTML")
		a.partial(w, r, status, "error", Page{Error: message})
		return
	}
	a.render(w, r, status, Page{View: "error-page", Title: "Something needs attention", Error: message})
}
func (a *App) fail(w http.ResponseWriter, r *http.Request, err error) {
	status, message := 500, "Something went wrong. Your changes may not have been saved; try the same submission again."
	switch {
	case errors.Is(err, sql.ErrNoRows):
		status, message = 404, "That item could not be found."
	case errors.Is(err, ErrForbidden):
		status, message = 403, "You do not have permission to do that."
	case errors.Is(err, ErrConflict):
		status, message = 409, sentence(err.Error())
	case errors.Is(err, ErrInvalid):
		status, message = 422, "Check the submitted details and try again."
	case errors.Is(err, ErrInvitation) || errors.Is(err, ErrUsername):
		status, message = 422, sentence(err.Error())
	default:
		slog.Error("request failed", "path", r.URL.Path, "error", err)
	}
	if strings.HasPrefix(r.URL.Path, "/api/") {
		http.Error(w, message, status)
		return
	}
	a.render(w, r, status, Page{View: "error-page", Title: "Something needs attention", Error: message})
}
func (a *App) json(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(value)
}
func routeID(r *http.Request) (int64, error) {
	value := r.PathValue("id")
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id < 1 || strconv.FormatInt(id, 10) != value {
		return 0, sql.ErrNoRows
	}
	return id, nil
}

var webURL = regexp.MustCompile(`(?i)https?://[^\s<>"']+`)

func linkify(value string) template.HTML {
	var out strings.Builder
	last := 0
	for _, index := range webURL.FindAllStringIndex(value, -1) {
		out.WriteString(template.HTMLEscapeString(value[last:index[0]]))
		raw := value[index[0]:index[1]]
		clean := strings.TrimRight(raw, ".,;!?)")
		u, err := url.Parse(clean)
		if err == nil && u.Hostname() != "" && u.User == nil && !strings.ContainsAny(clean, "\\\x00\r\n") {
			fmt.Fprintf(&out, `<a href="%s" rel="noreferrer noopener">%s</a>%s`, template.HTMLEscapeString(clean), template.HTMLEscapeString(clean), template.HTMLEscapeString(raw[len(clean):]))
		} else {
			out.WriteString(template.HTMLEscapeString(raw))
		}
		last = index[1]
	}
	out.WriteString(template.HTMLEscapeString(value[last:]))
	return template.HTML(out.String())
}

type rateLimiter struct {
	once   sync.Once
	shared *ratelimit.Window
}

func (l *rateLimiter) allow(key string) bool {
	l.once.Do(func() {
		l.shared = ratelimit.NewWindow(ratelimit.WindowConfig{Attempts: 20, Interval: 15 * time.Minute, Capacity: 4096, RejectNew: true})
	})
	ok, _ := l.shared.Allow(key)
	return ok
}
func (a *App) authAllowed(w http.ResponseWriter, r *http.Request) bool {
	if state(r).User != nil {
		a.redirect(w, r, "/")
		return false
	}
	if !a.limiter.allow(clientip.NetworkKey(a.resolver.Client(r))) {
		w.Header().Set("Retry-After", "900")
		a.render(w, r, 429, Page{View: "error-page", Title: "Take a little break", Error: "Too many attempts. Try again in 15 minutes."})
		return false
	}
	return true
}
func contentType(r *http.Request) string {
	value, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	return value
}
