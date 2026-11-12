package folkweave

import (
	"database/sql"
	"github.com/airencracken/comfylib/analytics/analyticstest"
	"path/filepath"
	"testing"
)

func TestSharedAnalyticsSQLiteContract(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "analytics.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	analyticstest.Run(t, db)
}
