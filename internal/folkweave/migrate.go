package folkweave

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"

	"github.com/airencracken/comfylib/twofactor"
)

const schemaVersion = 4

//go:embed schema_v2.sql
var schemaV2 string

//go:embed schema_v4.sql
var schemaV4 string

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
	if version < 3 {
		if err := twofactor.Install(ctx, tx); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `CREATE TRIGGER IF NOT EXISTS twofactor_invalidated_login AFTER UPDATE OF password_hash,disabled ON users
WHEN OLD.password_hash IS NOT NEW.password_hash OR OLD.disabled IS NOT NEW.disabled
BEGIN
 DELETE FROM twofactor_challenges WHERE user_id=NEW.id;
 DELETE FROM twofactor_secrets WHERE user_id=NEW.id AND last_step IS NULL;
END;`); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DROP TABLE schema_version;
CREATE TABLE schema_version(version INTEGER NOT NULL CHECK(version=3));
INSERT INTO schema_version VALUES(3);`); err != nil {
			return err
		}
	}
	if version < 4 {
		if _, err := tx.ExecContext(ctx, schemaV4); err != nil {
			return err
		}
	}
	return tx.Commit()
}
