package folkweave

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	_ "image/png"

	"github.com/airencracken/comfylib/ui"
	xdraw "golang.org/x/image/draw"
)

const MaxPhotoBytes = 5_000_000

func newPhotoID() (string, error) {
	var data [16]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(data[:]), nil
}

// NormalizePhoto decodes bounded JPEG/PNG input and publishes only a resized,
// metadata-free JPEG. Originals never become publicly addressable files.
func NormalizePhoto(data []byte, alt string) (Photo, error) {
	if len(data) == 0 || len(data) > MaxPhotoBytes || !validText(alt, 1000, false) {
		return Photo{}, errors.New("choose JPEG or PNG photos up to " + ui.Size(MaxPhotoBytes) + ", with descriptions up to 1,000 characters")
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "jpeg" && format != "png") || config.Width < 1 || config.Height < 1 || config.Width > 8192 || config.Height > 8192 || int64(config.Width)*int64(config.Height) > 20000000 {
		return Photo{}, errors.New("choose a JPEG or PNG photo up to 20 megapixels")
	}
	source, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return Photo{}, errors.New("this photo could not be read")
	}
	if format == "jpeg" {
		source = orient(source, jpegOrientation(data))
	}
	w, h := source.Bounds().Dx(), source.Bounds().Dy()
	if max(w, h) > 1920 {
		scale := float64(1920) / float64(max(w, h))
		w, h = max(1, int(float64(w)*scale)), max(1, int(float64(h)*scale))
	}
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(out, out.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	xdraw.CatmullRom.Scale(out, out.Bounds(), source, source.Bounds(), draw.Over, nil)
	var encoded bytes.Buffer
	if err = jpeg.Encode(&encoded, out, &jpeg.Options{Quality: 85}); err != nil {
		return Photo{}, err
	}
	if encoded.Len() > 8<<20 {
		return Photo{}, errors.New("this photo is too large after resizing; try a smaller one")
	}
	return Photo{Alt: alt, Width: w, Height: h, Data: encoded.Bytes()}, nil
}

// JPEG APP1 orientation is read with explicit bounds; no other EXIF is retained.
func jpegOrientation(data []byte) int {
	if len(data) < 4 || data[0] != 0xff || data[1] != 0xd8 {
		return 1
	}
	for pos := 2; pos+4 <= len(data); {
		if data[pos] != 0xff {
			return 1
		}
		marker := data[pos+1]
		pos += 2
		if marker == 0xda || marker == 0xd9 {
			return 1
		}
		if marker == 0xff {
			pos--
			continue
		}
		if marker == 0x01 || (marker >= 0xd0 && marker <= 0xd7) {
			continue
		}
		n := int(binary.BigEndian.Uint16(data[pos : pos+2]))
		if n < 2 || n > len(data)-pos {
			return 1
		}
		segment := data[pos+2 : pos+n]
		pos += n
		if marker != 0xe1 || len(segment) < 14 || !bytes.Equal(segment[:6], []byte("Exif\x00\x00")) {
			continue
		}
		tiff := segment[6:]
		var order binary.ByteOrder
		switch string(tiff[:2]) {
		case "II":
			order = binary.LittleEndian
		case "MM":
			order = binary.BigEndian
		default:
			return 1
		}
		if order.Uint16(tiff[2:4]) != 42 {
			return 1
		}
		offset := uint64(order.Uint32(tiff[4:8]))
		if offset+2 > uint64(len(tiff)) {
			return 1
		}
		count := int(order.Uint16(tiff[offset : offset+2]))
		offset += 2
		for i := 0; i < count; i++ {
			entry := offset + uint64(i)*12
			if entry+12 > uint64(len(tiff)) {
				return 1
			}
			v := tiff[entry : entry+12]
			if order.Uint16(v[:2]) == 0x112 && order.Uint16(v[2:4]) == 3 && order.Uint32(v[4:8]) == 1 {
				orientation := int(order.Uint16(v[8:10]))
				if orientation >= 1 && orientation <= 8 {
					return orientation
				}
				return 1
			}
		}
	}
	return 1
}
func orient(src image.Image, orientation int) image.Image {
	if orientation <= 1 || orientation > 8 {
		return src
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dw, dh := w, h
	if orientation >= 5 {
		dw, dh = h, w
	}
	out := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx, dy := x, y
			switch orientation {
			case 2:
				dx = w - 1 - x
			case 3:
				dx, dy = w-1-x, h-1-y
			case 4:
				dy = h - 1 - y
			case 5:
				dx, dy = y, x
			case 6:
				dx, dy = h-1-y, x
			case 7:
				dx, dy = h-1-y, w-1-x
			case 8:
				dx, dy = y, w-1-x
			}
			out.Set(dx, dy, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return out
}
