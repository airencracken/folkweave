package folkweave

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"strings"
	"testing"
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
