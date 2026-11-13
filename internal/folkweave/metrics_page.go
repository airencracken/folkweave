// SPDX-License-Identifier: AGPL-3.0-or-later
package folkweave

import (
	"github.com/airencracken/comfylib/admin"
	"net/http"
	"time"
)

// metrics shows aggregate instance health only. Folkweave does not record
// per-account sign-ins or per-item views, so the shared dashboard receives no
// activity summary and omits its activity section entirely.
func (a *App) metrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	metrics, err := a.store.Metrics(r.Context(), time.Now())
	if err != nil {
		a.fail(w, r, err)
		return
	}
	a.render(w, r, 200, Page{View: "metrics", Title: "Instance metrics", Overview: admin.View{Metrics: metrics, BackURL: "/settings"}})
}
