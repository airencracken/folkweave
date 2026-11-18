package folkweave

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
)

const schemaVersion = 2

//go:embed schema_v2.sql
var schemaV2 string

func initializeSchema(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var known bool
	if err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM sqlite_schema WHERE type='table' AND name='schema_version')").Scan(&known); err != nil {
		return err
	}
	count, version := 1, 1
	if known {
		if err := tx.QueryRowContext(ctx, "SELECT count(*),coalesce(min(version),0) FROM schema_version").Scan(&count, &version); err != nil {
			return err
		}
	}
	if count != 1 || version < 1 || version > schemaVersion {
		return errors.New("unsupported Folkweave schema version")
	}
	if version == 1 {
		if _, err := tx.ExecContext(ctx, schema); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, schemaV2); err != nil {
			return err
		}
	}
	return tx.Commit()
}
