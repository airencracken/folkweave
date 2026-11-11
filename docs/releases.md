# Install Folkweave

For first-run accounts, use [browser or command-line setup](setup.md). Run
`folkweave setup` for the browser key, or `folkweave create-owner --username NAME`
for a hidden terminal prompt.

The [GitHub releases](https://github.com/airencracken/folkweave/releases) provide
versioned Linux amd64 and arm64 archives, Debian packages, a source archive,
and a SHA-256 checksum manifest. The current release is **0.3.1**.

## Linux archive

Choose your architecture, download the archive and matching checksum manifest,
and run `sha256sum -c folkweave_0.3.1_checksums.txt --ignore-missing` in that directory.
Each archive contains the executable, license and dependency notices, README,
maintained guides, and service/proxy examples under `deploy/`.

Run the executable as an unprivileged account. For an archive installation,
create a private data directory and set `FOLKWEAVE_DATA_DIR` explicitly. Run
`folkweave --help` for commands, and `folkweave --version` to check the installed build.

## Debian or Ubuntu

```sh
sudo apt install ./folkweave_0.3.1_amd64.deb
sudo folkweave init --username owner --name "Your name" --title "Our group"
sudo systemctl enable --now folkweave
```

Use the arm64 package on an arm64 host. The package creates a dedicated service
account and a mode-0700 `/var/lib/folkweave`. Configuration lives in the mode-0600
`/etc/folkweave/folkweave.env`. Installation does not start or enable a fresh service.
An upgrade restarts an already-running service. Existing configuration and all
application data survive removal and purge.

## Gentoo

Add the [Comfyware overlay](https://github.com/airencracken/comfyware#add-the-overlay)
and install `www-apps/folkweave`. The overlay includes the service account, OpenRC
and systemd units, and offline Go dependencies. Configure `/etc/conf.d/folkweave`
for OpenRC or `/etc/folkweave/folkweave.env` for systemd. Run `folkweave init --username owner --name "Your name" --title "Our group"` as root after
installation; the administrative command re-executes as the service account.

For OpenRC, start the service for browser setup with `rc-service folkweave start`.
Add the configured service to a runlevel with `rc-update add folkweave default`.

## Containers and source

The [container guide](containers.md) covers pinned multi-platform images and
Docker Compose. Source builds require Go 1.26.11 or newer; `make build` resolves
only the modules pinned in `go.mod` and `go.sum`. There is no frontend build.

## Upgrades

Back up before replacing a binary or package. Stop the service, install the new
version, and start it again. Startup applies schema changes. Never open a newer
database with an older binary; restore a pre-upgrade snapshot into a new private
directory when rolling back. Keep exact package/image version pins.

## Rebuild an existing release tag

Maintainers can dispatch the Release workflow on `master` with `release_tag` set
to an existing version tag. The workflow validates the tag, exact commit, VERSION
where present, and reachability from master. It uses Go 1.26.11 and runs the full
native, package and container gates against that tag. Container revision labels
identify the checked-out source commit. Published releases and image tags remain
immutable; reruns may finish an unpublished draft.
