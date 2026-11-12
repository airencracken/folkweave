// SPDX-License-Identifier: AGPL-3.0-or-later
package folkweave

import (
	"context"
	"github.com/airencracken/comfylib/admin"
	"time"
)

// Metrics returns aggregate instance totals from one bounded read snapshot.
// Authorization is performed by the owner/admin HTTP handler before this call.
func (s *Store) Metrics(ctx context.Context, now time.Time) ([]admin.Metric, error) {
	return admin.Collect(ctx, s.db, []admin.Query{
		{Label: "Members", SQL: "SELECT count(*) FROM users WHERE disabled=0", Detail: "", Bytes: false},
		{Label: "Posts", SQL: "SELECT count(*) FROM posts", Detail: "", Bytes: false},
		{Label: "Replies", SQL: "SELECT count(*) FROM replies", Detail: "", Bytes: false},
		{Label: "Photos", SQL: "SELECT count(*) FROM photos", Detail: "", Bytes: false},
		{Label: "Photo bytes", SQL: "SELECT coalesce(sum(length(data)),0) FROM photos", Detail: "Original photographs; previews excluded.", Bytes: true},
		{Label: "New posts", SQL: "SELECT count(*) FROM posts WHERE created_at>=? AND created_at<=?", Detail: "Last 7 days", Bytes: false, Args: []any{now.Add(-7 * 24 * time.Hour).UnixMilli(), now.UnixMilli()}},
		{Label: "Database size", SQL: "SELECT (SELECT page_count FROM pragma_page_count)*(SELECT page_size FROM pragma_page_size)", Bytes: true, Detail: "Main database allocation; temporary logs, cached media and backups are separate."},
	})
}
