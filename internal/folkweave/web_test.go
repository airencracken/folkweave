package folkweave

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/airencracken/comfylib/token"
)

type testClient struct {
	app     *App
	cookies map[string]*http.Cookie
	ip      string
}

func webFixture(t *testing.T) (*Store, *App, *testClient) {
	t.Helper()
	s := testStore(t)
	app, err := NewApp(s, Config{BaseURL: "http://example.test"})
	if err != nil {
		t.Fatal(err)
	}
	return s, app, &testClient{app: app, cookies: map[string]*http.Cookie{}}
}
func (c *testClient) request(method, path string, values url.Values, headers map[string]string) *httptest.ResponseRecorder {
	var body io.Reader
	if values != nil {
		body = strings.NewReader(values.Encode())
	}
	request := httptest.NewRequest(method, "http://example.test"+path, body)
	request.RemoteAddr = "192.0.2.50:12345"
	if c.ip != "" {
		request.RemoteAddr = c.ip + ":12345"
	}
	if values != nil {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for _, cookie := range c.cookies {
		request.AddCookie(cookie)
	}
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	w := httptest.NewRecorder()
	c.app.ServeHTTP(w, request)
	for _, cookie := range w.Result().Cookies() {
		if cookie.MaxAge < 0 {
			delete(c.cookies, cookie.Name)
		} else {
			c.cookies[cookie.Name] = cookie
		}
	}
	return w
}

var csrfPattern = regexp.MustCompile(`name="csrf" value="([a-f0-9]+)"`)

func (c *testClient) csrf(t *testing.T) string {
	t.Helper()
	path := "/login"
	if c.cookies[c.app.cookieName("session")] != nil {
		path = "/account"
	}
	w := c.request("GET", path, nil, nil)
	match := csrfPattern.FindStringSubmatch(w.Body.String())
	if len(match) != 2 {
		t.Fatalf("missing CSRF form; status %d", w.Code)
	}
	return match[1]
}
func (c *testClient) signIn(t *testing.T, name string) {
	t.Helper()
	csrf := c.csrf(t)
	w := c.request("POST", "/login", url.Values{"username": {name}, "password": {"a-long-test-password"}, "csrf": {csrf}}, nil)
	if w.Code != 303 {
		t.Fatalf("login status %d: %s", w.Code, w.Body.String())
	}
}
func (c *testClient) form(t *testing.T, path string, values url.Values, htmx bool) *httptest.ResponseRecorder {
	t.Helper()
	values.Set("csrf", c.csrf(t))
	headers := map[string]string{}
	if htmx {
		headers["HX-Request"] = "true"
	}
	return c.request("POST", path, values, headers)
}

func TestRoutesPrivacyAndSecurityHeaders(t *testing.T) {
	s, _, c := webFixture(t)
	id, _ := s.CreatePost(testContext, 1, "Private family photograph", testKey(), []Photo{testPhoto(t)})
	post, _ := s.Post(testContext, 1, id)
	for _, path := range []string{"/", "/saved", "/people", "/account", "/invitations", "/settings", post.URL(), "/people/1"} {
		w := c.request("GET", path, nil, nil)
		if w.Code != 303 || w.Header().Get("Location") != "/login" || strings.Contains(w.Body.String(), "Private family") {
			t.Fatalf("private route %s leaked: %d", path, w.Code)
		}
	}
	w := c.request("GET", post.Photos[0].URL(), nil, nil)
	if w.Code != 404 {
		t.Fatal("anonymous photo route", w.Code)
	}
	for _, path := range []string{"/api/v1/me", "/api/v1/posts", "/api/v1/posts/1"} {
		if w = c.request("GET", path, nil, nil); w.Code != 401 {
			t.Fatal(path, w.Code)
		}
	}
	c.signIn(t, "owner")
	for _, path := range []string{"/", "/saved", "/people", "/account", "/invitations", "/settings", post.URL(), post.URL() + "/edit", "/people/1", "/api/v1/me", "/api/v1/posts", "/api/v1/posts/1", post.Photos[0].URL()} {
		w = c.request("GET", path, nil, nil)
		if w.Code != 200 {
			t.Fatalf("route %s status %d: %s", path, w.Code, w.Body.String())
		}
		if w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Referrer-Policy") != "no-referrer" || strings.Contains(w.Header().Get("Content-Security-Policy"), "unsafe-eval") {
			t.Fatal("privacy headers missing", path)
		}
	}
	w = c.request("GET", "/", nil, nil)
	if !strings.Contains(w.Body.String(), `hx-history="false"`) {
		t.Fatal("private HTMX history caching enabled")
	}
	w = c.request("GET", "/static/app.js", nil, nil)
	if w.Code != 200 || strings.Contains(w.Header().Get("Content-Security-Policy"), "unsafe-inline") {
		t.Fatal("static route or CSP missing")
	}
}

func TestAccountAndSettingsNavigation(t *testing.T) {
	s, _, c := webFixture(t)
	c.signIn(t, "owner")
	for _, page := range []struct{ path, heading string }{
		{"/settings", "Settings"}, {"/account", "Account"}, {"/invitations", "Invitations"},
	} {
		w := c.request("GET", page.path, nil, nil)
		if w.Code != 200 || !strings.Contains(w.Body.String(), "<h1>"+page.heading+"</h1>") {
			t.Fatalf("missing clear page heading on %s: %d", page.path, w.Code)
		}
		for _, control := range []string{`href="/account"`, `>Account</a>`, `href="/settings">Settings</a>`, `href="/account/export">Export your data</a>`} {
			if !strings.Contains(w.Body.String(), control) {
				t.Fatal("missing navigation control", page.path, control)
			}
		}
	}
	w := c.form(t, "/settings", url.Values{"title": {"Our group"}, "about": {"Keep shared photos private."}}, false)
	if w.Code != 303 {
		t.Fatal("settings save failed", w.Code, w.Body.String())
	}
	settings, err := s.Settings(testContext)
	if err != nil || settings.Title != "Our group" || settings.About != "Keep shared photos private." {
		t.Fatal("settings were not saved", settings, err)
	}
	for _, asset := range []struct{ path, contentType string }{
		{"/static/loom.webp", "image/webp"}, {"/static/favicon-32.png", "image/png"},
	} {
		w := c.request("GET", asset.path, nil, nil)
		if w.Code != 200 || w.Header().Get("Content-Type") != asset.contentType {
			t.Fatal("mascot asset not served correctly", asset.path, w.Code, w.Header())
		}
	}
}

func TestCSRFOriginAndMethodBoundaries(t *testing.T) {
	_, _, c := webFixture(t)
	c.signIn(t, "owner")
	values := url.Values{"body": {"No CSRF"}, "submission": {testKey()}}
	w := c.request("POST", "/posts", values, nil)
	if w.Code != 403 {
		t.Fatal("missing CSRF accepted")
	}
	values.Set("csrf", c.csrf(t))
	w = c.request("POST", "/posts", values, map[string]string{"Origin": "https://attacker.invalid", "Sec-Fetch-Site": "cross-site"})
	if w.Code != 403 {
		t.Fatal("cross-site write accepted", w.Code)
	}
	values["csrf"] = []string{values.Get("csrf"), values.Get("csrf")}
	if w = c.request("POST", "/posts", values, nil); w.Code != 403 {
		t.Fatal("duplicate CSRF accepted")
	}
	for _, path := range []string{"/logout", "/posts", "/posts/1/delete", "/replies/1/delete"} {
		if w = c.request("GET", path, nil, nil); w.Code != 405 {
			t.Fatal("mutation has GET route", path, w.Code)
		}
	}
	request := httptest.NewRequest("GET", "http://unknown.test/", nil)
	w = httptest.NewRecorder()
	c.app.ServeHTTP(w, request)
	if w.Code != 400 {
		t.Fatal("untrusted Host accepted")
	}
}

func TestPostReplyRefreshAndRetryContract(t *testing.T) {
	s, _, c := webFixture(t)
	c.signIn(t, "owner")
	key := testKey()
	values := url.Values{"body": {"<script>bad()</script> A good day https://example.org/hello?a=1&b=2"}, "submission": {key}}
	w := c.form(t, "/posts", values, true)
	if w.Code != 201 || w.Header().Get("HX-Trigger") != "folkweave:posted" || !strings.Contains(w.Body.String(), `hx-swap-oob="outerHTML"`) {
		t.Fatal("post did not refresh timeline", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "<script>bad()") || !strings.Contains(w.Body.String(), "&lt;script&gt;") {
		t.Fatal("post HTML not escaped")
	}
	w = c.form(t, "/posts", values, true)
	if w.Code != 201 {
		t.Fatal("retry rejected", w.Code)
	}
	var n int
	s.db.QueryRow("SELECT count(*) FROM posts").Scan(&n)
	if n != 1 {
		t.Fatal("duplicate post")
	}
	reply := url.Values{"body": {"This reply should appear immediately"}, "submission": {testKey()}}
	for i := 0; i < 2; i++ {
		w = c.form(t, "/posts/1/replies", reply, true)
		if w.Code != 200 || !strings.Contains(w.Body.String(), reply.Get("body")) || !strings.Contains(w.Body.String(), `id="discussion"`) {
			t.Fatal("reply did not return fresh discussion", w.Code)
		}
	}
	s.db.QueryRow("SELECT count(*) FROM replies").Scan(&n)
	if n != 1 {
		t.Fatal("duplicate reply")
	}
	bad := url.Values{"body": {""}, "submission": {testKey()}}
	w = c.form(t, "/posts/1/replies", bad, true)
	if w.Code != 422 || w.Header().Get("HX-Retarget") != "#reply-error" {
		t.Fatal("invalid reply replaced draft", w.Code, w.Header())
	}
}

func TestOwnerRoutesAndSessionRevocation(t *testing.T) {
	s, a, c := webFixture(t)
	member := testMember(t, s, "member")
	c.signIn(t, "member")
	for _, path := range []string{"/settings", "/invitations"} {
		w := c.request("GET", path, nil, nil)
		if w.Code != 403 {
			t.Fatal("owner page accessible", path, w.Code)
		}
	}
	w := c.form(t, "/invitations", url.Values{"uses": {"1"}}, false)
	if w.Code != 403 {
		t.Fatal("member created invitation")
	}
	if err := s.SetDisabled(testContext, 1, member, true); err != nil {
		t.Fatal(err)
	}
	w = c.request("GET", "/", nil, nil)
	if w.Code != 303 {
		t.Fatal("disabled session still active")
	}
	owner := &testClient{app: a, cookies: map[string]*http.Cookie{}}
	owner.signIn(t, "owner")
	w = owner.form(t, "/logout", url.Values{}, false)
	if w.Code != 303 || owner.cookies[a.cookieName("session")] != nil {
		t.Fatal("logout did not revoke session")
	}
}

func TestInvitationJoinAndOneTimeLinkDisclosure(t *testing.T) {
	s, a, c := webFixture(t)
	c.signIn(t, "owner")
	w := c.form(t, "/invitations", url.Values{"uses": {"1"}}, false)
	if w.Code != 201 {
		t.Fatal(w.Code)
	}
	pattern := regexp.MustCompile(`/join\?invite=([a-f0-9]{64})`)
	match := pattern.FindStringSubmatch(w.Body.String())
	if len(match) != 2 {
		t.Fatal("invitation link absent")
	}
	secret := match[1]
	w = c.request("GET", "/invitations", nil, nil)
	if strings.Contains(w.Body.String(), secret) {
		t.Fatal("stored invitation token was disclosed again")
	}
	guest := &testClient{app: a, cookies: map[string]*http.Cookie{}}
	csrf := guest.csrf(t)
	w = guest.request("POST", "/join", url.Values{"csrf": {csrf}, "username": {"new_member"}, "name": {"New Member"}, "password": {"another-long-password"}, "invite": {secret}}, nil)
	if w.Code != 303 || guest.cookies[a.cookieName("session")] == nil {
		t.Fatal("join failed", w.Code, w.Body.String())
	}
	var uses int
	s.db.QueryRow("SELECT uses FROM invitations WHERE token_hash=?", token.Hash(secret)).Scan(&uses)
	if uses != 1 {
		t.Fatal("invitation not consumed")
	}
}

func TestMultipartPhotoRoutesAndFailedDraft(t *testing.T) {
	s, _, c := webFixture(t)
	c.signIn(t, "owner")
	key := testKey()
	photo := testPhoto(t)
	post := func(data []byte, body string) *httptest.ResponseRecorder {
		var buffer bytes.Buffer
		writer := multipart.NewWriter(&buffer)
		for k, v := range map[string]string{"csrf": c.csrf(t), "submission": key, "body": body, "photo_alt": "An orange corner"} {
			writer.WriteField(k, v)
		}
		file, _ := writer.CreateFormFile("photos", "picture.jpg")
		file.Write(data)
		writer.Close()
		r := httptest.NewRequest("POST", "http://example.test/posts", &buffer)
		r.RemoteAddr = "192.0.2.50:12345"
		r.Header.Set("Content-Type", writer.FormDataContentType())
		r.Header.Set("HX-Request", "true")
		for _, cookie := range c.cookies {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		c.app.ServeHTTP(w, r)
		return w
	}
	w := post([]byte("not a real photo"), "Draft worth keeping")
	if w.Code != 422 || w.Header().Get("HX-Retarget") != "#composer-error" || strings.Contains(w.Body.String(), "<form") {
		t.Fatal("error replaced composer", w.Code, w.Header(), w.Body.String())
	}
	var count int
	s.db.QueryRow("SELECT count(*) FROM posts").Scan(&count)
	if count != 0 {
		t.Fatal("failed photo produced post")
	}
	w = post(photo.Data, "Draft worth keeping")
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	p, err := s.Post(testContext, 1, 1)
	if err != nil || len(p.Photos) != 1 || p.Photos[0].Alt != "An orange corner" {
		t.Fatal("photo not attached", err)
	}
	w = c.request("GET", p.Photos[0].URL(), nil, nil)
	if w.Code != 200 || w.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatal("photo route broken")
	}
	c.form(t, "/logout", url.Values{}, false)
	if w = c.request("GET", p.Photos[0].URL(), nil, nil); w.Code != 404 {
		t.Fatal("photo accessible after logout")
	}
}

func TestAPIContractAndPrivateExport(t *testing.T) {
	s, _, c := webFixture(t)
	member := testMember(t, s, "member")
	s.CreatePost(testContext, 1, "Owner's private post", testKey(), []Photo{testPhoto(t)})
	s.CreatePost(testContext, member, "Member's own post", testKey(), nil)
	c.signIn(t, "member")
	w := c.request("GET", "/api/v1/posts", nil, nil)
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	var contract struct {
		Posts  []Post `json:"posts"`
		Next   string `json:"next"`
		Anchor int64  `json:"anchor"`
	}
	decoder := json.NewDecoder(w.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&contract); err != nil {
		t.Fatal(err)
	}
	if len(contract.Posts) != 2 || contract.Posts[0].Author.ID != member || contract.Anchor != 2 {
		t.Fatal("unexpected API shape", contract)
	}
	for _, post := range contract.Posts {
		if post.Photos == nil {
			t.Fatal("photos must be an array")
		}
	}
	w = c.request("GET", "/account/export", nil, nil)
	archive, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if len(archive.File) != 1 || archive.File[0].Name != "account.json" {
		t.Fatal("other member's photographs exported")
	}
	reader, _ := archive.File[0].Open()
	data, _ := io.ReadAll(reader)
	reader.Close()
	if bytes.Contains(data, []byte("Owner's private post")) || bytes.Contains(data, []byte("password")) || !bytes.Contains(data, []byte("Member's own post")) {
		t.Fatal("export ownership contract violated")
	}
}

func TestMalformedInputsAndUnsafeLinks(t *testing.T) {
	_, _, c := webFixture(t)
	c.signIn(t, "owner")
	for _, path := range []string{"/posts/-1", "/posts/01", "/posts/999999999999999999999", "/photos/not-an-id", "/people/0"} {
		w := c.request("GET", path, nil, nil)
		if w.Code != 404 {
			t.Fatal(path, w.Code)
		}
	}
	for _, text := range []string{`<img src=x onerror=alert(1)>`, `javascript:alert(1)`, `https://user:secret@example.com/`, `https://example.com/" onmouseover="alert(1)`} {
		rendered := string(linkify(text))
		if strings.Contains(rendered, "<img") || strings.Contains(rendered, `href="javascript:`) || strings.Contains(rendered, `href="https://user:`) {
			t.Fatal("unsafe link", rendered)
		}
	}
	values := url.Values{"body": {"a", "b"}, "submission": {testKey()}, "csrf": {c.csrf(t)}}
	w := c.request("POST", "/posts", values, nil)
	if w.Code != 400 {
		t.Fatal("duplicate body accepted", w.Code)
	}
}

func TestSecureCookieAndAuthenticationLimits(t *testing.T) {
	s := testStore(t)
	a, err := NewApp(s, Config{BaseURL: "https://example.test"})
	if err != nil {
		t.Fatal(err)
	}
	c := &testClient{app: a, cookies: map[string]*http.Cookie{}}
	c.signIn(t, "owner")
	cookie := c.cookies[a.cookieName("session")]
	if cookie == nil || !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode || !strings.HasPrefix(cookie.Name, "__Host-") {
		t.Fatal("session cookie not confined")
	}
	guest := &testClient{app: a, cookies: map[string]*http.Cookie{}, ip: "192.0.2.51"}
	csrf := guest.csrf(t)
	for i := 0; i < 20; i++ {
		w := guest.request("POST", "/login", url.Values{"csrf": {csrf}, "username": {"missing_" + strconv.Itoa(i)}, "password": {"bad"}}, nil)
		if w.Code != 422 {
			t.Fatal(i, w.Code)
		}
	}
	w := guest.request("POST", "/login", url.Values{"csrf": {csrf}, "username": {"missing"}, "password": {"bad"}}, nil)
	if w.Code != 429 {
		t.Fatal("auth limit absent", fmt.Sprint(w.Code))
	}
}
