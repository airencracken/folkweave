package folkweave

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"time"

	"github.com/airencracken/comfylib/token"
)

// SeedDemo is used only by the disposable, loopback-only demo command.
func SeedDemo(ctx context.Context, s *Store) error {
	hash, err := HashPassword("folkweave-demo-only")
	if err != nil {
		return err
	}
	if err = s.Bootstrap(ctx, "demo", hash, "Ari", "The Sunday Folks"); err != nil {
		return err
	}
	if err = s.Configure(ctx, 1, "The Sunday Folks", "A place for the everyday things. Put the kettle on and stay a while.", string(defaultPalette)); err != nil {
		return err
	}
	invite, err := s.NewInvitation(ctx, 1, 2)
	if err != nil {
		return err
	}
	jules, err := s.Register(ctx, "jules", hash, "Jules", invite)
	if err != nil {
		return err
	}
	rowan, err := s.Register(ctx, "rowan", hash, "Rowan", invite)
	if err != nil {
		return err
	}
	key, _ := token.New()
	first, err := s.CreatePost(ctx, rowan, "Anyone fancy a walk on Saturday? The woods should be lovely after all this rain. I am thinking late morning, with somewhere for lunch afterwards.", key, nil)
	if err != nil {
		return err
	}
	key, _ = token.New()
	if _, err = s.AddReply(ctx, jules, first, "Count me in. I can bring the flask.", key); err != nil {
		return err
	}
	key, _ = token.New()
	if _, err = s.CreatePost(ctx, jules, "A small good thing from this week: finally finished the book that has been sitting by the bed since spring.\n\nNo hurry was the right hurry.", key, nil); err != nil {
		return err
	}
	photos := []Photo{}
	for index, alt := range []string{"An illustrated woodland path beneath a pale autumn sky.", "An illustrated view of warm hills and trees."} {
		photo, err := NormalizePhoto(demoLandscape(index), alt)
		if err != nil {
			return err
		}
		photos = append(photos, photo)
	}
	key, _ = token.New()
	last, err := s.CreatePost(ctx, 1, "Took the long way home this morning. Glad I did.", key, photos)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, "UPDATE posts SET created_at=?-(?-id)*3600000", time.Now().UnixMilli(), last)
	return err
}
func demoLandscape(variant int) []byte {
	picture := image.NewRGBA(image.Rect(0, 0, 900, 620))
	sky := color.RGBA{221, 227, 207, 255}
	hill := color.RGBA{139, 158, 112, 255}
	near := color.RGBA{78, 112, 82, 255}
	path := color.RGBA{210, 186, 142, 255}
	if variant == 1 {
		sky = color.RGBA{238, 220, 184, 255}
		hill = color.RGBA{171, 159, 100, 255}
		near = color.RGBA{100, 122, 87, 255}
	}
	for y := 0; y < 620; y++ {
		for x := 0; x < 900; x++ {
			c := sky
			if y > 240+(x/9)%90 {
				c = hill
			}
			if y > 380-(x/12)%80 {
				c = near
			}
			if y > 390 && x > 330-(y-390)/3 && x < 410+(y-390)/2 {
				c = path
			}
			if (x > 80 && x < 108 || x > 720 && x < 746) && y > 150 {
				c = color.RGBA{82, 76, 51, 255}
			}
			picture.SetRGBA(x, y, c)
		}
	}
	var out bytes.Buffer
	png.Encode(&out, picture)
	return out.Bytes()
}
