// SPDX-License-Identifier: AGPL-3.0-or-later
package folkweave

import (
	"context"
	"github.com/airencracken/comfylib/admin"
	"time"
)

// weekStart is midnight UTC six days before now: a seven-calendar-day window
// that includes today.
func weekStart(now time.Time) time.Time {
	return now.UTC().Truncate(24 * time.Hour).Add(-6 * 24 * time.Hour)
}

// Metrics returns aggregate instance totals from one bounded read snapshot.
// Every value is a count or a size; no query names an account or an item.
// Authorization is performed by the owner/admin HTTP handler before this call.
func (s *Store) Metrics(ctx context.Context, now time.Time) ([]admin.Metric, error) {
	return admin.Collect(ctx, s.db, []admin.Query{
		{Label: "Members", SQL: "SELECT count(*) FROM users WHERE disabled=0", Detail: "", Bytes: false},
		{Label: "Suspended members", SQL: "SELECT count(*) FROM users WHERE disabled=1", Detail: "", Bytes: false},
		{Label: "Open invitations", SQL: "SELECT count(*) FROM invitations WHERE revoked=0 AND expires_at>? AND uses<max_uses", Detail: "", Bytes: false, Args: []any{now.UnixMilli()}},
		{Label: "Posts", SQL: "SELECT count(*) FROM posts", Detail: "", Bytes: false},
		{Label: "Replies", SQL: "SELECT count(*) FROM replies", Detail: "", Bytes: false},
		{Label: "Photos", SQL: "SELECT count(*) FROM photos", Detail: "", Bytes: false},
		{Label: "Photo bytes", SQL: "SELECT coalesce(sum(length(data)),0) FROM photos", Detail: "Original photographs; previews excluded.", Bytes: true},
		{Label: "New posts", SQL: "SELECT count(*) FROM posts WHERE created_at>=? AND created_at<=?", Detail: "Past 7 UTC days", Bytes: false, Args: []any{weekStart(now).UnixMilli(), now.UnixMilli()}},
		{Label: "Database size", SQL: "SELECT (SELECT page_count FROM pragma_page_count)*(SELECT page_size FROM pragma_page_size)", Bytes: true, Detail: "Main database allocation; temporary logs, cached media and backups are separate."},
	})
}
