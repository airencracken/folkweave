# Folkweave releases

## 0.3.4

- Make administration visible in the main navigation for owners.
- Resolve shared tooling on fresh container CI runners and verify it with the
  module proxy disabled before running the release gates.

- Import the suite helpers directly from published Comfylib 0.3.0 and use its
  embedded styles and browser controls.
- Replace the local OpenID Connect protocol adapter with the separately versioned
  Comfylib OIDC module 0.1.0. Existing local session and permission hooks remain
  application-owned.
- Run service configuration, deployment, release, artifact scanning, mutation and
  container checks directly from the pinned shared module. Retain application
  configuration and both existing backup/restore lifecycle profiles.
- Add integration tests that verify shared asset routes and exact published bytes.

This maintenance release adds no application database migration or new default
integration. Back up your data before upgrading. When upgrading from older
versions, follow the earlier release notes for any migrations you have not run.

## 0.3.1

- Forward optional app navigation and community sign-in settings through OpenRC
  and Docker Compose. Native templates document disabled defaults. Regression
  tests check literal values and reject missing service exports.
- Add an optional Apps menu and consistent keyboard, focus and account controls.
- Add explicitly linked community sign-in using OpenID Connect, state, nonce and
  PKCE. Existing local accounts and passwords remain available; accounts are never
  created or linked by matching email addresses. Configure this only when wanted.
- Use published Comfylib v0.2.0 and require Go 1.26.11 or newer for source builds.
- Upgrade golang.org/x/crypto to v0.59.0 and scan the actual release executables
  for vulnerable dependencies before publication.

- Serve cached 320, 640 and 960 pixel photograph previews with responsive images
  and lazy loading. Full-size normalized photos remain available when opened.
- Bound concurrent preview generation and preserve photo authorization and
  deletion guarantees. The additive photo_previews table stores derived images.

Read [Suite integration](suite.md) for configuration and security boundaries.
Back up the stopped data directory before upgrading. Shared sign-in creates an
additive identity-binding table when configured. Keep the pre-upgrade backup;
use it when returning to older binaries rather than opening a migrated database
with an older application. Publishing a release does not change running services.


# Folkweave 0.3.0

- Add an optional Apps menu and consistent keyboard, focus and account controls.
- Add explicitly linked community sign-in using OpenID Connect, state, nonce and
  PKCE. Existing local accounts and passwords remain available; accounts are never
  created or linked by matching email addresses. Configure this only when wanted.
- Use published Comfylib v0.2.0 and require Go 1.26.11 or newer for source builds.
- Upgrade golang.org/x/crypto to v0.59.0 and scan the actual release executables
  for vulnerable dependencies before publication.

- Serve cached 320, 640 and 960 pixel photograph previews with responsive images
  and lazy loading. Full-size normalized photos remain available when opened.
- Bound concurrent preview generation and preserve photo authorization and
  deletion guarantees. The additive photo_previews table stores derived images.

Read [Suite integration](suite.md) for configuration and security boundaries.
Back up the stopped data directory before upgrading. Shared sign-in creates an
additive identity-binding table when configured. Keep the pre-upgrade backup;
use it when returning to older binaries rather than opening a migrated database
with an older application. Publishing a release does not change running services.

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
