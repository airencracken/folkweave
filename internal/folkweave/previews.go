package folkweave

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"image"
	"image/jpeg"

	"golang.org/x/image/draw"
)

// PhotoPreview is a rebuildable derivative of the metadata-free shared copy.
// The foreign key prevents stale previews surviving a deleted photograph.
func (s *Store) PhotoPreview(ctx context.Context, id string, width int) ([]byte, error) {
	if width != 320 && width != 640 && width != 960 {
		return nil, ErrInvalid
	}
	var cached []byte
	err := s.db.QueryRowContext(ctx, "SELECT data FROM photo_previews WHERE photo_id=? AND width=?", id, width).Scan(&cached)
	if err == nil || !errors.Is(err, sql.ErrNoRows) {
		return cached, err
	}
	photo, err := s.Photo(ctx, id)
	if err != nil {
		return nil, err
	}
	preview, err := photoPreview(photo.Data, width)
	if err != nil {
		return nil, err
	}
	result, err := s.db.ExecContext(ctx, `INSERT INTO photo_previews(photo_id,width,data)
 SELECT id,?,? FROM photos WHERE id=? ON CONFLICT(photo_id,width) DO NOTHING`, width, preview, id)
	if err != nil {
		return nil, err
	}
	// Read the published row, including when another request got there first.
	if _, err = result.RowsAffected(); err != nil {
		return nil, err
	}
	err = s.db.QueryRowContext(ctx, "SELECT data FROM photo_previews WHERE photo_id=? AND width=?", id, width).Scan(&cached)
	return cached, err
}

func photoPreview(data []byte, width int) ([]byte, error) {
	config, kind, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || kind != "jpeg" || config.Width < 1 || config.Height < 1 || max(config.Width, config.Height) > 1920 {
		return nil, ErrInvalid
	}
	if config.Width <= width {
		return data, nil
	}
	source, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	height := max(1, config.Height*width/config.Width)
	out := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.ApproxBiLinear.Scale(out, out.Bounds(), source, source.Bounds(), draw.Src, nil)
	var encoded bytes.Buffer
	if err = jpeg.Encode(&encoded, out, &jpeg.Options{Quality: 80}); err != nil {
		return nil, err
	}
	return encoded.Bytes(), nil
}
