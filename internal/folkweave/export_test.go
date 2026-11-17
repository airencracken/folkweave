package folkweave

import (
	"archive/zip"
	"bytes"
	"net/http"
	"testing"
)

func TestAuditExportFailsWhenPhotoContentCannotBeRead(t *testing.T) {
	store, _, client := webFixture(t)
	id, err := store.CreatePost(testContext, 1, "A photograph worth keeping", testKey(), []Photo{testPhoto(t)})
	if err != nil {
		t.Fatal(err)
	}
	client.signIn(t, "owner")
	const failPhotoRead = `
		ALTER TABLE photos RENAME TO fixture_photos;
		CREATE VIEW photos AS SELECT id, post_id, position, alt, width, height,
			abs(-9223372036854775808) AS data FROM fixture_photos;`
	if _, err := store.db.Exec(failPhotoRead); err != nil {
		t.Fatal(err)
	}
	post, err := store.Post(testContext, 1, id)
	if err != nil || len(post.Photos) != 1 {
		t.Fatalf("fixture must leave photo metadata readable: %v", err)
	}
	if _, err := store.Photo(testContext, post.Photos[0].ID); err == nil {
		t.Fatal("fixture must fail reading photo content")
	}
	response := client.request("GET", "/account/export", nil, nil)
	if response.Code == http.StatusOK {
		archive, err := zip.NewReader(bytes.NewReader(response.Body.Bytes()), int64(response.Body.Len()))
		if err != nil {
			t.Fatalf("export returned HTTP 200 with an unreadable ZIP: %v", err)
		}
		for _, file := range archive.File {
			if file.Name == "photos/"+post.Photos[0].ID+".jpg" {
				t.Fatal("fixture unexpectedly exported the photograph")
			}
		}
		t.Fatal("export returned HTTP 200 with a valid ZIP that silently omitted the photograph")
	}
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("expected HTTP 500 for a failed export, got %d", response.Code)
	}
}
