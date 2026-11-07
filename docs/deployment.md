# Host Folkweave

Native services bind to `127.0.0.1:8084` and use `/var/lib/folkweave`. A single
installation has one active server; a lock prevents a second server from opening
that instance. The data directory is private (0700) and the database is 0600.
Do not serve the data directory from your reverse proxy.

Configure the public HTTPS origin before inviting members:

```sh
FOLKWEAVE_BASE_URL="https://folkweave.example.org"
FOLKWEAVE_TRUSTED_PROXIES="127.0.0.1/32,::1/128"
```

Only trust proxies that you control. Examples for Caddy, nginx, and Apache are
included under `deploy/`. Keep the upstream listener private and terminate TLS
at your proxy. Native defaults use 8084; containers use port 8080 internally.

OpenRC accepts ordinary `FOLKWEAVE_*=value` assignments in `/etc/conf.d/folkweave`;
`export` is optional. The init script passes the supported settings to the service.

## Services and account commands

The Debian and Gentoo packages use `/usr/bin/folkweave`. Source service examples
use `/usr/local/bin/folkweave`. Use a dedicated `folkweave` user and group, and create
its private state directory before starting a portable service installation.
Systemd and OpenRC pass configuration through the environment. Their examples
use a 0077 umask, run without root privileges, and preserve private state.

Run owner creation, password recovery, backup and restore commands as the same
account that owns the database. Installed administrative commands invoked as root
resolve the service settings and re-run as its configured account. Unmanaged
root administration is refused. Portable installations should use an explicitly
configured unprivileged account and `FOLKWEAVE_DATA_DIR`.

```sh
folkweave init --username owner --name "Your name" --title "Our group"
folkweave set-password --username member
folkweave backup --output /var/lib/folkweave/manual-snapshot
```

Never put a password in command-line arguments. Interactive entry disables
terminal echo and asks for confirmation. Automation can use --password-stdin.

## Logs and shutdown

Systemd writes to the journal (`journalctl -u folkweave`). OpenRC writes to
`/var/log/folkweave.log`; the included logrotate rule retains eight compressed
weekly logs. Logs and backups may contain private data; protect them.

A normal SIGTERM closes the listener, waits for background work, and closes the
database. Check `/healthz` for readiness. Do not use process existence as the
only readiness signal. Start the service and use browser setup, or create the first owner through the CLI.

See [backup and recovery](automatic-backups.md) and [installation/upgrades](releases.md).

## Creating owners from the command line

Releases from 0.2.0 accept `folkweave create-owner --username NAME`,
using the same hidden password confirmation and service-account re-execution as
`init`. It initializes a fresh gathering or adds another owner to an existing
one. Existing usernames are rejected, and existing settings, accounts, and
sessions stay intact. `--name` sets the new owner's display name; `--title` sets
the gathering name only when its settings are first created.

For the published 0.1.0 binary, use `folkweave init --username NAME` to create the
first owner. The `init` command remains supported and refuses an already
initialized gathering. Use `set-password` to reset an existing account's password.

## Configuration format

Use double quotes for every value, including numbers and booleans. No `export`
prefix is needed in service configuration files:

```sh
FOLKWEAVE_BASE_URL="https://folkweave.example.org"
FOLKWEAVE_BACKUP_KEEP="7"
```

OpenRC reads `/etc/conf.d/folkweave`; systemd reads
`/etc/folkweave/folkweave.env`. Edit the file for the service manager you run,
then restart the service. The other manager's file is independent. Existing
bare or single-quoted values remain valid, and upgrades preserve local settings.

The [shared configuration convention](https://github.com/airencracken/comfylib/blob/master/docs/configuration.md)
covers quoting, escaping, and how the examples are checked.
