# Folkweave 0.2.1

- Standardize shipped service configs and documented environment examples on
  `KEY="value"`, including numbers, booleans, and empty values.
- Identify the service manager and installed config path in every template.
- Check source and generated package configs, literal quoting, and OpenRC
  environment inheritance during `make check`.

Existing configuration values remain supported and local files are preserved.
No database migration is introduced.

# Folkweave 0.2.0

- Add browser first-owner setup through `folkweave setup`, with a private host key,
  confirmed passwords, and setup closing once an owner exists.
- Align `create-owner` and `set-password` across Comfyware: hidden confirmed input
  by default, plus `--password-prompt`, `--password-stdin`, `--password-file`,
  and `--data-dir`. Password changes end earlier signed-in sessions.
- Use published Comfylib v0.1.8 and retain existing account command aliases.

# Folkweave 0.1.1

- Fix OpenRC configuration inheritance: existing plain assignments now reach the
  service without requiring `export`, including the public origin and proxy trust.

- Add `folkweave create-owner --username NAME` for first-time setup and additional
  owner accounts. It preserves existing settings and rejects existing usernames.
  The `init` command remains available for first-time setup; existing 0.1.0
  installations can use it immediately.

# Folkweave 0.1.0

The first public release of Folkweave. This is an early release for small,
self-hosted installations; read the current scope and limitations in the README.

- Invite-only accounts, chronological posts with up to four photos, replies, profiles and private bookmarks.
- Normalized private photographs, account exports, moderation and retry-safe submissions.
- Verified automatic and manual backups, restoration, service configuration,
  and account recovery using Comfylib v0.1.7.
- Linux amd64/arm64 archives, Debian packages, source and checksums, versioned
  multi-platform container images, and Gentoo overlay packaging.
- Private service accounts, loopback native defaults, HTTPS proxy examples,
  and tested installation and container lifecycles.

One installation has one shared audience. Parent-managed accounts and parental approval controls remain future work. Original photograph uploads are not retained.

[Installation](releases.md) · [Containers](containers.md) · [Hosting](deployment.md) · [Backups](automatic-backups.md)
