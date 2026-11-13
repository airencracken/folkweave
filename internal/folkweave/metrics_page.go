// SPDX-License-Identifier: AGPL-3.0-or-later
package folkweave

import (
	"github.com/airencracken/comfylib/admin"
	"github.com/airencracken/comfylib/analytics"
	"net/http"
	"time"
)

// metrics shows instance totals plus site health and security counts. None of
// these name an account or an item.
func (a *App) metrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	metrics, err := a.store.Metrics(r.Context(), time.Now())
	if err != nil {
		a.fail(w, r, err)
		return
	}
	health, err := analytics.Collect(r.Context(), a.store.db, "", time.Now())
	if err != nil {
		a.fail(w, r, err)
		return
	}
	a.render(w, r, 200, Page{View: "metrics", Title: "Instance metrics", Overview: admin.View{Metrics: metrics, Activity: &health, BackURL: "/settings"}})
}
