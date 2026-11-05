package folkweave

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"io/fs"
	"strings"
	"testing"

	"golang.org/x/image/webp"
)

func TestEmbeddedAssetProvenanceAndIsolation(t *testing.T) {
	data, err := assets.ReadFile("static/vendor.json")
	if err != nil {
		t.Fatal(err)
	}
	var entries []struct{ Package, Version, License, Source, SHA256, File string }
	if err = json.Unmarshal(data, &entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatal("expected two pinned browser libraries")
	}
	for _, entry := range entries {
		data, err := assets.ReadFile("static/" + entry.File)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != entry.SHA256 {
			t.Fatal("browser asset differs from recorded source", entry.File)
		}
		license, err := assets.ReadFile("static/" + entry.File + ".LICENSE")
		if err != nil || len(license) < 40 || entry.Version == "" || entry.License == "" || !strings.HasPrefix(entry.Source, "https://registry.npmjs.org/") {
			t.Fatal("asset provenance incomplete", entry.File, err)
		}
	}
	if _, err = fs.Stat(assets, "static/app.test.cjs"); err == nil {
		t.Fatal("test scripts were embedded")
	}
	if _, err = fs.Stat(assets, "schema.sql"); err == nil {
		t.Fatal("private schema embedded into web assets")
	}
}

func TestMascotAssets(t *testing.T) {
	for _, asset := range []struct {
		name   string
		size   int
		budget int
		decode func(io.Reader) (image.Image, error)
	}{
		{"loom.webp", 256, 25000, webp.Decode},
		{"favicon-32.png", 32, 5000, png.Decode},
	} {
		t.Run(asset.name, func(t *testing.T) {
			data, err := assets.ReadFile("static/" + asset.name)
			if err != nil {
				t.Fatal(err)
			}
			if len(data) > asset.budget {
				t.Fatalf("asset exceeds %d bytes: %d", asset.budget, len(data))
			}
			picture, err := asset.decode(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if picture.Bounds() != image.Rect(0, 0, asset.size, asset.size) {
				t.Fatal("unexpected dimensions", picture.Bounds())
			}
			for _, point := range []image.Point{{0, 0}, {asset.size - 1, 0}, {0, asset.size - 1}, {asset.size - 1, asset.size - 1}} {
				if _, _, _, alpha := picture.At(point.X, point.Y).RGBA(); alpha != 0 {
					t.Fatal("mascot lacks transparent margins", point)
				}
			}
			if _, _, _, alpha := picture.At(asset.size/2, asset.size/2).RGBA(); alpha == 0 {
				t.Fatal("mascot has no visible center")
			}
		})
	}
}
