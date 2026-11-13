// SPDX-License-Identifier: AGPL-3.0-or-later
package folkweave

import (
	"bytes"
	"mime/multipart"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/airencracken/comfylib/autobackup"
)

var internalWording = []string{"MiB", "72 bytes", "Unexpected attachment", "Send each form field", "supported photo width", "normalized photo"}

func assertPlain(t *testing.T, label, body string) {
	t.Helper()
	for _, word := range internalWording {
		if strings.Contains(body, word) {
			t.Fatalf("%s shows internal wording %q", label, word)
		}
	}
}

func TestSentence(t *testing.T) {
	for in, want := range map[string]string{
		"":                             "",
		"use a shorter password":       "Use a shorter password.",
		"already done.":                "Already done.",
		"  écrit  ":                    "Écrit.",
		"this changed; reload and try": "This changed; reload and try.",
		"why?":                         "Why?",
	} {
		if got := sentence(in); got != want {
			t.Fatalf("sentence(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPasswordMessagesAvoidByteCounts(t *testing.T) {
	if err := ValidateCredentials("member", "short"); err == nil || strings.Contains(err.Error(), "bytes") {
		t.Fatal(err)
	}
	err := ValidateCredentials("member", strings.Repeat("é", 40))
	if err == nil || err.Error() != "use a shorter password" {
		t.Fatal("long password", err)
	}
	if err = ValidateCredentials("member", strings.Repeat("a", 72)); err != nil {
		t.Fatal("72-byte password rejected", err)
	}
}

func TestJoinShowsPlainPasswordGuidance(t *testing.T) {
	_, _, guest := webFixture(t)
	page := guest.request("GET", "/join?invite=x", nil, nil).Body.String()
	assertPlain(t, "join form", page)
	w := guest.form(t, "/join", url.Values{"invite": {"x"}, "name": {"A"}, "username": {"member"}, "password": {strings.Repeat("é", 40)}}, false)
	if w.Code != 422 || !strings.Contains(w.Body.String(), "Use a shorter password.") {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestMalformedSubmissionsGetOneGenericMessage(t *testing.T) {
	_, a, owner := webFixture(t)
	owner.signIn(t, "owner")
	for _, htmx := range []bool{false, true} {
		w := owner.request("POST", "/posts", url.Values{"csrf": {owner.csrf(t)}, "body": {"one", "two"}, "submission": {testKey()}}, map[string]string{"HX-Request": map[bool]string{true: "true"}[htmx]})
		if w.Code != 400 || !strings.Contains(w.Body.String(), malformedSubmission) {
			t.Fatal("repeated field", htmx, w.Code, w.Body.String())
		}
		assertPlain(t, "repeated field", w.Body.String())
	}
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	form.WriteField("csrf", owner.csrf(t))
	form.WriteField("submission", testKey())
	form.WriteField("body", "Hello")
	part, _ := form.CreateFormFile("smuggled", "x.jpg")
	part.Write([]byte("not a photo"))
	form.Close()
	request := httptest.NewRequest("POST", "http://example.test/posts", &body)
	request.Header.Set("Content-Type", form.FormDataContentType())
	for _, cookie := range owner.cookies {
		request.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, request)
	if w.Code != 422 || !strings.Contains(w.Body.String(), malformedSubmission) {
		t.Fatal("unexpected attachment", w.Code, w.Body.String())
	}
	assertPlain(t, "unexpected attachment", w.Body.String())
}

func TestPhotoWidthAndComposerUsePlainWording(t *testing.T) {
	s, _, owner := webFixture(t)
	owner.signIn(t, "owner")
	id, err := s.CreatePost(testContext, 1, "Width fixture", testKey(), []Photo{testPhoto(t)})
	if err != nil {
		t.Fatal(err)
	}
	post, _ := s.Post(testContext, 1, id)
	for _, width := range []string{"7", "-1", "640&width=320", "%00", strings.Repeat("9", 400)} {
		w := owner.request("GET", post.Photos[0].URL()+"?width="+width, nil, nil)
		if w.Code != 400 {
			t.Fatal(width, w.Code)
		}
		assertPlain(t, "photo width", w.Body.String())
	}
	timeline := owner.request("GET", "/", nil, nil).Body.String()
	if !strings.Contains(timeline, "5 MB each") {
		t.Fatal("composer size limit missing")
	}
	assertPlain(t, "timeline", timeline)
}

func failedBackupRunner(t *testing.T, s *Store, raw string) *autobackup.Runner {
	t.Helper()
	data := t.TempDir()
	config, err := autobackup.Load("folkweave", data, func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(config.Directory, 0700); err != nil {
		t.Fatal(err)
	}
	state := `{"version":1,"last_attempt":"2026-01-02T03:04:05Z","last_success":"0001-01-01T00:00:00Z","last_error":` + "\"" + raw + "\"" + `}`
	if err = os.WriteFile(filepath.Join(config.Directory, ".automatic-backup.json"), []byte(state), 0600); err != nil {
		t.Fatal(err)
	}
	runner, err := autobackup.New(config, s.Snapshot, nil)
	if err != nil {
		t.Fatal(err)
	}
	if runner.Status().LastError != raw {
		t.Fatal("fixture state not loaded", runner.Status())
	}
	return runner
}

func TestOwnerSeesBackupSummaryInsteadOfRawError(t *testing.T) {
	s, a, owner := webFixture(t)
	raw := "open /var/lib/folkweave/backups/stage-91: permission denied"
	a.Backups = failedBackupRunner(t, s, raw)
	owner.signIn(t, "owner")
	for _, path := range []string{"/settings", "/admin/metrics"} {
		body := owner.request("GET", path, nil, nil).Body.String()
		if strings.Contains(body, raw) || strings.Contains(body, "/var/lib") {
			t.Fatal(path, "shows the raw backup error")
		}
		if !strings.Contains(body, backupProblem) {
			t.Fatal(path, "missing backup summary")
		}
	}
	if got := ownerBackupStatus(autobackup.Status{}); got.LastError != "" {
		t.Fatal("summary invented for a healthy backup", got)
	}
}
