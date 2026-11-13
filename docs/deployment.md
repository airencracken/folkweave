# Host Folkweave

Native installs listen on `127.0.0.1:8084` and keep data in `/var/lib/folkweave`.
The data directory must be private (mode 0700) and the database 0600. Only one
server can use a data directory at a time. Never serve the data directory
through your proxy.

## Configure

Set the public HTTPS origin before you invite anyone. Trust only proxies you
run:

```sh
FOLKWEAVE_BASE_URL="https://folkweave.example.org"
FOLKWEAVE_TRUSTED_PROXIES="127.0.0.1/32,::1/128"
```

OpenRC reads `/etc/conf.d/folkweave` and systemd reads
`/etc/folkweave/folkweave.env`. Edit the one for your service manager and
restart the service. Write every value as `KEY="value"`, numbers included; no
`export` is needed.

| Setting | Default |
| --- | --- |
| `FOLKWEAVE_DATA_DIR` | `/var/lib/folkweave` for installed services, otherwise `./data` |
| `FOLKWEAVE_LISTEN` | `127.0.0.1:8084` |
| `FOLKWEAVE_BASE_URL` | The listen address over HTTP |
| `FOLKWEAVE_TRUSTED_PROXIES` | None |

Backup settings are in [backups and recovery](automatic-backups.md), and the
Apps menu and community sign-in settings in [Comfyware connections](suite.md).

Proxy examples for Caddy, nginx, and Apache are in `deploy/`. Terminate TLS at
the proxy and keep Folkweave's listener private.

## Account commands

Run account, backup, and restore commands as the account that owns the data.
On an installed service, running them with `sudo` finds the service settings
and switches to the service account. Plain root is refused.

```sh
folkweave create-owner --username owner --name "Your name" --title "Our group"
folkweave set-password --username member
folkweave backup --output /var/lib/folkweave/manual-snapshot
```

`create-owner` creates the first owner, or adds another owner without changing
the group's settings. `--title` sets the group name only when the group is
created. Existing usernames are refused; use `set-password` to reset a password.

Never put a password on the command line. The commands ask for it twice
without echoing, and automation can use `--password-stdin`. On a portable
install, pass `--data-dir` or set `FOLKWEAVE_DATA_DIR`.

## Services

Debian and Gentoo packages install `/usr/bin/folkweave`; the examples in
`deploy/` use `/usr/local/bin/folkweave`. Run the service as a dedicated
`folkweave` user and group, and create its private data directory first.

## Logs and shutdown

systemd logs go to the journal (`journalctl -u folkweave`). OpenRC writes
`/var/log/folkweave.log`, and the included logrotate rule keeps eight weekly
compressed logs. Logs and backups can hold private data, so protect them.

On SIGTERM the server stops accepting connections, finishes background work,
and closes the database. Use `/healthz` to check readiness.

See [install and upgrade](releases.md) and [setup](setup.md).
