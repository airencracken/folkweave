# Install Folkweave

[GitHub releases](https://github.com/airencracken/folkweave/releases) have
Linux amd64 and arm64 archives, Debian packages, a source archive, and a
SHA-256 checksum file. The current release is **0.7.2**.

After installing, create the first owner with [setup](setup.md).

## Linux archive

Download the archive for your architecture and the checksum file, then check
them in that directory:

```sh
sha256sum -c folkweave_0.7.2_checksums.txt --ignore-missing
```

The archive holds the program, licenses, the README, these guides, and service
and proxy examples in `deploy/`. Run Folkweave as an unprivileged account with
a private data directory, and set `FOLKWEAVE_DATA_DIR` to it.

## Debian or Ubuntu

```sh
sudo apt install ./folkweave_0.7.2_amd64.deb
sudo folkweave create-owner --username owner --name "Your name" --title "Our group"
sudo systemctl enable --now folkweave
```

Use the arm64 package on arm64. The package creates a `folkweave` service
account, a private `/var/lib/folkweave`, and `/etc/folkweave/folkweave.env`.
It does not start a new install. An upgrade restarts a running service.
Removing or purging the package keeps your configuration and data.

## Gentoo

Add the [Comfyware overlay](https://github.com/airencracken/comfyware#add-the-overlay)
and install `www-apps/folkweave`. Configure `/etc/conf.d/folkweave` (OpenRC)
or `/etc/folkweave/folkweave.env` (systemd), then create the owner:

```sh
sudo folkweave create-owner --username owner --name "Your name" --title "Our group"
rc-service folkweave start
rc-update add folkweave default
```

## Containers and source

See [Docker Compose](containers.md). Source builds need Go 1.26.11 or later
and use only the modules pinned in `go.sum`; there is no frontend build step.

## Upgrade

Back up, stop the service, install the new version, and start it. Startup
upgrades the database. An older version cannot open an upgraded database, so to
roll back, restore your pre-upgrade backup into a new directory. Read the
[release notes](release-notes.md) first.

## Rebuild a release

Maintainers can run the Release workflow on `master` with `release_tag` set to
an existing tag. It checks that the tag is on `master`, rebuilds, and runs the
full checks. Published releases and image tags are never replaced.
