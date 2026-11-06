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
