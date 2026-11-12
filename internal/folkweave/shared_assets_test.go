// SPDX-License-Identifier: AGPL-3.0-or-later
package folkweave

import (
	"github.com/airencracken/comfylib/ui"
	"io/fs"
	"strings"
	"testing"
)

func TestSharedSuiteAssetsAreServedFromPinnedLibrary(t *testing.T) {
	_, _, c := webFixture(t)
	for _, asset := range []struct{ name, contentType string }{{"accounts.css", "text/css"}, {"components.css", "text/css"}, {"suite.js", "javascript"}} {
		path := "/static/comfyware/" + asset.name
		response := c.request("GET", path, nil, nil)
		want, err := fs.ReadFile(ui.Assets(), asset.name)
		if err != nil {
			t.Fatal(err)
		}
		if response.Code != 200 || response.Body.String() != string(want) || !strings.Contains(response.Header().Get("Content-Type"), asset.contentType) {
			t.Fatal("shared asset route differs from pinned library", path, response.Code)
		}
	}
	for _, path := range []string{"/static/comfyware/assets.go", "/static/comfyware/uitest/controls.mjs", "/static/comfyware/missing.css"} {
		if response := c.request("GET", path, nil, nil); response.Code != 404 {
			t.Fatal("private asset exposed", path, response.Code)
		}
	}
}
