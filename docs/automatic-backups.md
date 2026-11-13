# Backups and recovery

The server backs up every day, starting about a minute after it starts. Each
backup is a verified copy of the SQLite database, which holds everything,
photos included. Backups are compressed archives in `backups/` inside the data
directory. Owners see the latest result under **Admin**, on Settings and
Metrics; details of a failure are in the server log.

| Setting | Default |
| --- | --- |
| `FOLKWEAVE_BACKUP_INTERVAL` | `24h`; `0` turns automatic backups off |
| `FOLKWEAVE_BACKUP_KEEP` | `7` archives |
| `FOLKWEAVE_BACKUP_MAX_BYTES` | `10737418240` (10 GiB) for all archives |
| `FOLKWEAVE_BACKUP_COPY_MAX_BYTES` | `5368709120` (5 GiB) per uncompressed copy |

Archives use zstd when it is installed and gzip otherwise. Restoring a zstd
archive needs the `zstd` program.

## Back up by hand

This works while the server runs and creates a new directory holding
`folkweave.db`:

```sh
folkweave backup --output /var/lib/folkweave/manual-snapshot
```

## Restore

Restore into a new directory; `restore` refuses one that exists and checks the
copy first. For an automatic archive, unpack it somewhere private and use the
`folkweave.db` inside.

```sh
folkweave restore --from /private/snapshot/folkweave.db --data-dir /private/folkweave-restored
```

Run it as the service account. Stop the service, point `FOLKWEAVE_DATA_DIR` at
the restored directory, and start it again. Check that you can sign in and see
your posts before you delete the old copy.

Keep copies on another machine and practice restoring them. Backups hold
password hashes, sessions, and every member's posts, so protect them. A
member's export is not a backup of the group.

Do not copy only the live database file while the server runs. Use `backup`,
or copy the whole data directory with the service stopped.
