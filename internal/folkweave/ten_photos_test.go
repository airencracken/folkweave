package folkweave

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTenPhotoUploadAndLimit(t *testing.T) {
	s, app, client := webFixture(t)
	client.signIn(t, "owner")
	photo := testPhoto(t)
	upload := func(count int) *httptest.ResponseRecorder {
		t.Helper()
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		for field, value := range map[string]string{"csrf": client.csrf(t), "submission": testKey(), "body": "An afternoon outside"} {
			if err := writer.WriteField(field, value); err != nil {
				t.Fatal(err)
			}
		}
		for index := range count {
			if err := writer.WriteField("photo_alt", fmt.Sprintf("Photo %d description", index+1)); err != nil {
				t.Fatal(err)
			}
			file, err := writer.CreateFormFile("photos", fmt.Sprintf("photo-%d.jpg", index+1))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := file.Write(photo.Data); err != nil {
				t.Fatal(err)
			}
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequest(http.MethodPost, "http://example.test/posts", &body)
		request.Header.Set("Content-Type", writer.FormDataContentType())
		for _, cookie := range client.cookies {
			request.AddCookie(cookie)
		}
		response := httptest.NewRecorder()
		app.ServeHTTP(response, request)
		return response
	}

	if response := upload(10); response.Code != http.StatusSeeOther {
		t.Fatalf("ten-photo upload: %d %s", response.Code, response.Body)
	}
	if response := upload(11); response.Code != http.StatusUnprocessableEntity || !strings.Contains(response.Body.String(), "Choose up to 10") {
		t.Fatalf("eleven-photo upload: %d %s", response.Code, response.Body)
	}
	feed, err := s.Posts(t.Context(), 1, "", false, 0)
	if err != nil || len(feed.Posts) != 1 {
		t.Fatalf("rejected upload changed the timeline: %+v, %v", feed, err)
	}
	response := client.request(http.MethodGet, "/api/v1"+feed.Posts[0].URL(), nil, nil)
	var post Post
	if response.Code != http.StatusOK || json.NewDecoder(response.Body).Decode(&post) != nil || len(post.Photos) != 10 {
		t.Fatalf("unexpected photo API contract: %d, %+v", response.Code, post.Photos)
	}
	ids := make(map[string]bool)
	for index, attachment := range post.Photos {
		if ids[attachment.ID] || attachment.Alt != fmt.Sprintf("Photo %d description", index+1) {
			t.Fatalf("wrong photo identity or order at %d: %+v", index, attachment)
		}
		ids[attachment.ID] = true
	}
	last := post.Photos[9]
	if response := client.request(http.MethodGet, last.URL(), nil, nil); response.Code != http.StatusOK || response.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("tenth photo route returned %d", response.Code)
	}
	if _, err := s.db.Exec("UPDATE photos SET position=10 WHERE id=?", last.ID); err == nil {
		t.Fatal("schema accepted an eleventh position")
	}
}

func TestPhotoSchemaUpgradeAndRollback(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "data")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "folkweave.db")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	legacy, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { legacy.Close() })
	if _, err := legacy.Exec(schema); err != nil {
		t.Fatal(err)
	}
	const id = "0123456789abcdef0123456789abcdef"
	photo := testPhoto(t)
	for _, statement := range []string{
		"INSERT INTO users(id,username,password_hash,name,role,created_at) VALUES(1,'owner','fixture','Owner','owner',0)",
		"INSERT INTO posts(id,user_id,body,created_at,submission_key,fingerprint) VALUES(1,1,'Original',0,'key','print')",
		"CREATE TABLE photos_v1(conflicting_name TEXT)",
	} {
		if _, err := legacy.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := legacy.Exec("INSERT INTO photos VALUES(?,1,3,?,?,?,?)", id, photo.Alt, photo.Data, photo.Width, photo.Height); err != nil {
		t.Fatal(err)
	}
	preview, err := photoPreview(photo.Data, 320)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.Exec("INSERT INTO photo_previews VALUES(?,320,?)", id, preview); err != nil {
		t.Fatal(err)
	}

	if store, err := Open(dir); err == nil {
		store.Close()
		t.Fatal("migration ignored a conflicting table")
	}
	var version, addedColumns int
	if err := legacy.QueryRow("SELECT version FROM schema_version").Scan(&version); err != nil || version != 1 {
		t.Fatalf("failed migration advanced the version: %d, %v", version, err)
	}
	if err := legacy.QueryRow("SELECT count(*) FROM pragma_table_info('posts') WHERE name LIKE 'repost_%'").Scan(&addedColumns); err != nil || addedColumns != 0 {
		t.Fatalf("failed migration left %d repost columns: %v", addedColumns, err)
	}
	if _, err := legacy.Exec("DROP TABLE photos_v1"); err != nil {
		t.Fatal(err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}

	for range 2 {
		s, err := Open(dir)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { s.Close() })
		if err := s.Check(t.Context()); err != nil {
			t.Fatal(err)
		}
		got, err := s.Photo(t.Context(), id)
		if err != nil || got.Alt != photo.Alt || !bytes.Equal(got.Data, photo.Data) {
			t.Fatalf("upgraded photo differs: %+v, %v", got, err)
		}
		var cached []byte
		if err := s.db.QueryRow("SELECT data FROM photo_previews WHERE photo_id=? AND width=320", id).Scan(&cached); err != nil || !bytes.Equal(cached, preview) {
			t.Fatalf("migration lost the cached preview: %v", err)
		}
		if err := s.db.QueryRow("SELECT version FROM schema_version").Scan(&version); err != nil || version != schemaVersion {
			t.Fatalf("wrong schema version: %d, %v", version, err)
		}
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
