// SPDX-License-Identifier: AGPL-3.0-or-later
package folkweave

import (
	"context"
	"database/sql"
	"errors"

	"github.com/airencracken/comfylib/analytics"
)

// retiredActivityTable held per-account sign-ins and per-item views recorded
// by Folkweave 0.5.0 and 0.6.0. analytics.Initialize also drops it; dropping it
// here first, with secure_delete on, overwrites the freed pages so the old
// account and content IDs do not stay in the file. Safe to run on every open.
const retiredActivityTable = "comfy_analytics_activity"

func retireActivity(ctx context.Context, db *sql.DB) error {
	var present int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_schema WHERE type='table' AND name=?", retiredActivityTable).Scan(&present); err != nil {
		return err
	}
	if present == 0 {
		return nil
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	// Overwrite freed pages so removed account and content identifiers do not
	// linger in the database file after the tables are dropped.
	if _, err = conn.ExecContext(ctx, "PRAGMA secure_delete=ON"); err != nil {
		return err
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return errors.Join(err, restoreSecureDelete(conn))
	}
	if err = analytics.RemoveDetail(ctx, tx); err != nil {
		return errors.Join(err, tx.Rollback(), restoreSecureDelete(conn))
	}
	if err = tx.Commit(); err != nil {
		return errors.Join(err, restoreSecureDelete(conn))
	}
	return restoreSecureDelete(conn)
}

func restoreSecureDelete(conn *sql.Conn) error {
	_, err := conn.ExecContext(context.Background(), "PRAGMA secure_delete=OFF")
	return err
}
