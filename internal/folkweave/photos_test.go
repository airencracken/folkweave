package folkweave

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

func TestNormalizeJPEGOrientationAndMetadataRemoval(t *testing.T) {
	picture := image.NewRGBA(image.Rect(0, 0, 12, 8))
	var plain bytes.Buffer
	jpeg.Encode(&plain, picture, nil)
	for _, orientation := range []uint16{1, 2, 3, 4, 5, 6, 7, 8} {
		var tiff bytes.Buffer
		tiff.WriteString("II")
		binary.Write(&tiff, binary.LittleEndian, uint16(42))
		binary.Write(&tiff, binary.LittleEndian, uint32(8))
		binary.Write(&tiff, binary.LittleEndian, uint16(1))
		for _, n := range []uint16{0x112, 3} {
			binary.Write(&tiff, binary.LittleEndian, n)
		}
		binary.Write(&tiff, binary.LittleEndian, uint32(1))
		binary.Write(&tiff, binary.LittleEndian, orientation)
		binary.Write(&tiff, binary.LittleEndian, uint16(0))
		binary.Write(&tiff, binary.LittleEndian, uint32(0))
		segment := append([]byte("Exif\x00\x00"), tiff.Bytes()...)
		var data bytes.Buffer
		data.Write([]byte{0xff, 0xd8, 0xff, 0xe1})
		binary.Write(&data, binary.BigEndian, uint16(len(segment)+2))
		data.Write(segment)
		data.Write(plain.Bytes()[2:])
		photo, err := NormalizePhoto(data.Bytes(), "An oriented picture")
		if err != nil {
			t.Fatal(orientation, err)
		}
		wantW, wantH := 12, 8
		if orientation >= 5 {
			wantW, wantH = 8, 12
		}
		if photo.Width != wantW || photo.Height != wantH {
			t.Fatal("orientation dimensions", orientation, photo.Width, photo.Height)
		}
		if bytes.Contains(photo.Data, []byte("Exif")) {
			t.Fatal("EXIF retained")
		}
		if _, err = jpeg.Decode(bytes.NewReader(photo.Data)); err != nil {
			t.Fatal("output not a JPEG", err)
		}
	}
}
func TestEveryOrientationRetainsEveryPixel(t *testing.T) {
	source := image.NewRGBA(image.Rect(3, 5, 7, 8))
	colors := map[color.Color]bool{}
	for y := 5; y < 8; y++ {
		for x := 3; x < 7; x++ {
			c := color.RGBA{uint8(x * 20), uint8(y * 20), uint8(x + y), 255}
			source.Set(x, y, c)
			colors[c] = true
		}
	}
	for orientation := 1; orientation <= 8; orientation++ {
		out := orient(source, orientation)
		found := map[color.Color]bool{}
		b := out.Bounds()
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				found[out.At(x, y)] = true
			}
		}
		if len(found) != len(colors) {
			t.Fatal("orientation lost pixels", orientation)
		}
		for c := range colors {
			if !found[c] {
				t.Fatal("color lost", orientation)
			}
		}
	}
}
func TestPhotoBoundsAndTruncation(t *testing.T) {
	for _, input := range [][]byte{nil, []byte("<svg onload=bad()>"), {0xff, 0xd8, 0xff, 0xe1, 0xff, 0xff}, bytes.Repeat([]byte{1}, MaxPhotoBytes+1)} {
		if _, err := NormalizePhoto(input, ""); err == nil {
			t.Fatal("bad photo accepted")
		}
		jpegOrientation(input)
	}
	var png bytes.Buffer
	png.Write([]byte("\x89PNG\r\n\x1a\n"))
	binary.Write(&png, binary.BigEndian, uint32(13))
	chunk := []byte("IHDR")
	var size bytes.Buffer
	binary.Write(&size, binary.BigEndian, uint32(9000))
	binary.Write(&size, binary.BigEndian, uint32(9000))
	size.Write([]byte{8, 2, 0, 0, 0})
	chunk = append(chunk, size.Bytes()...)
	png.Write(chunk)
	binary.Write(&png, binary.BigEndian, crc32.ChecksumIEEE(chunk))
	if _, err := NormalizePhoto(png.Bytes(), ""); err == nil {
		t.Fatal("pixel bomb accepted")
	}
}
func FuzzJPEGOrientation(f *testing.F) {
	f.Add([]byte{0xff, 0xd8, 0xff, 0xe1, 0x00, 0x02})
	f.Add([]byte("garbage"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 65536 {
			return
		}
		value := jpegOrientation(data)
		if value < 1 || value > 8 {
			t.Fatal("invalid orientation", value)
		}
	})
}
