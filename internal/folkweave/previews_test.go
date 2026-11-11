package folkweave

import (
	"bytes"
	"database/sql"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"strings"
	"testing"

	"github.com/airencracken/comfylib/token"
)

func previewFixture(t testing.TB) []byte {
	t.Helper()
	source := image.NewRGBA(image.Rect(0, 0, 1920, 1280))
	for y := range 1280 {
		for x := range 1920 {
			source.SetRGBA(x, y, color.RGBA{uint8(x / 3), uint8(y / 3), uint8((x + y) / 7), 255})
		}
	}
	var data bytes.Buffer
	if err := jpeg.Encode(&data, source, &jpeg.Options{Quality: 85}); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

func TestPhotoPreviewDimensionsPrivacyAndDeletion(t *testing.T) {
	s, _, client := webFixture(t)
	raw := previewFixture(t)
	key, _ := token.New()
	id, err := s.CreatePost(testContext, 1, "", key, []Photo{{Data: raw, Width: 1920, Height: 1280}})
	if err != nil {
		t.Fatal(err)
	}
	post, err := s.Post(testContext, 1, id)
	if err != nil {
		t.Fatal(err)
	}
	photo := post.Photos[0]
	if got := client.request("GET", photo.URL()+"?width=320", nil, nil); got.Code != 404 {
		t.Fatal("preview was accessible before sign-in", got.Code)
	}
	client.signIn(t, "owner")
	for _, width := range []int{320, 640, 960} {
		data, err := s.PhotoPreview(testContext, photo.ID, width)
		if err != nil {
			t.Fatal(err)
		}
		config, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil || config.Width != width || config.Height != width*1280/1920 || len(data) >= len(raw) {
			t.Fatal("preview did not reduce dimensions and bytes", config, err)
		}
		cached, err := s.PhotoPreview(testContext, photo.ID, width)
		if err != nil || !bytes.Equal(cached, data) {
			t.Fatal("published preview changed", err)
		}
	}
	for _, query := range []string{"width=-1", "width=100000", "width=320&width=640", "width=%3Cscript%3E"} {
		if got := client.request("GET", photo.URL()+"?"+query, nil, nil); got.Code < 400 {
			t.Fatal("hostile width accepted", query, got.Code)
		}
	}
	response := client.request("GET", photo.URL()+"?width=320", nil, nil)
	if response.Code != 200 || response.Header().Get("Content-Type") != "image/jpeg" || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("preview route contract", response.Code, response.Header())
	}
	if _, err = s.db.Exec("DELETE FROM posts WHERE id=?", id); err != nil {
		t.Fatal(err)
	}
	if _, err = s.PhotoPreview(testContext, photo.ID, 320); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("deleted photo was resurrected", err)
	}
	var n int
	if err = s.db.QueryRow("SELECT count(*) FROM photo_previews").Scan(&n); err != nil || n != 0 {
		t.Fatal("deletion did not cascade", n, err)
	}
}

func TestResponsivePhotoCandidatesMatchActualWidths(t *testing.T) {
	for _, width := range []int{1, 100, 320, 500, 640, 960, 1920} {
		photo := Photo{ID: strings.Repeat("a", 32), Width: width}
		if width < 320 && strings.Contains(photo.Srcset(), "320w") {
			t.Fatal("small photo advertised an enlarged candidate")
		}
	}
	for _, data := range [][]byte{nil, []byte("not an image")} {
		if _, err := photoPreview(data, 320); err == nil {
			t.Fatal("invalid image accepted")
		}
	}
}

func BenchmarkPhotoPreview(b *testing.B) {
	data := previewFixture(b)
	b.ReportMetric(float64(len(data)), "source-bytes")
	for _, width := range []int{320, 640, 960} {
		b.Run(string(rune('0'+width/320)), func(b *testing.B) {
			preview, err := photoPreview(data, width)
			if err != nil {
				b.Fatal(err)
			}
			b.ReportMetric(float64(len(preview)), "preview-bytes")
			b.ReportAllocs()
			for b.Loop() {
				if _, err := photoPreview(data, width); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
