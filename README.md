# Folkweave

<img src="internal/folkweave/static/loom.webp" alt="Folkweave’s little wooden loom" width="144" height="144" align="right">

A private timeline for your people. A thought, a photograph, a bit of your day.

Folkweave is a small, self-hosted Comfyware app for friends, family, or another
group who choose each other. Each installation has its own accounts and
invite-only membership. Everyone in the gathering can see its earlier posts.

Current release: **[0.3.4](https://github.com/airencracken/folkweave/releases/tag/v0.3.4)**.

<br clear="right">

[Comfyware](https://comfyware.org/) · [Project page](https://comfyware.org/folkweave/) · [Help and bugs](https://github.com/airencracken/folkweave/issues)

Use a current Go security patch when building: Go 1.26.11 or later on the
1.26 series, or Go 1.27.4 or later on the 1.27 series.

## Folkweave 0.6.0

Account settings now offer the same **Times and dates** control used across the
five Comfyware applications. Open `/account`, choose an IANA timezone such as
`America/Los_Angeles`, and select **Save timezone**. UTC is the default. Your
choice is saved for your account across sessions and devices. **Use this device's
timezone** fills the field when JavaScript is available; saving remains explicit.
Manual selection and local date rendering also work without JavaScript.

Comfylib 0.6.0 owns validation, formatting, the account control, browser detection
and bounded template caching. Dates handle daylight saving and calendar boundaries.
Admin dashboard sign-in and backup instants follow your preference; reporting
days remain UTC. Stored timestamps, API responses and machine-readable timestamps
retain their existing UTC contract. Account isolation, native permissions and CSRF
checks apply to every preference change.

Startup adds a separately versioned display preference namespace. Application
migration and export format versions are unchanged. Back up the stopped data
directory before upgrading and retain that snapshot for a downgrade.

## Folkweave 0.5.0

Native admin metrics now use one shared dashboard with identical typography, spacing, cards, tables and reporting definitions across the apps. Successful local and community sign-ins and authorized content opens are recorded by Comfylib 0.5.0. Views count once per account, item and UTC day; thumbnails, background assets, failed requests and signed-out visits do not count.

Only active administrators (called owners in this app where applicable) can read the dashboard or its account sign-in summaries. Ordinary members, guests and moderators cannot read analytics. The dashboard shows the past seven UTC days, when collection began, daily totals, recent sign-ins by account, and most-viewed content. Recording begins after upgrade; no earlier history is inferred. No IP addresses or browser identifiers are recorded. Activity is retained for ninety UTC days, pruned at startup and daily while serving, and removed when its account is deleted.

The analytics tables are additive and separately versioned by Comfylib; existing application data and application migration versions are unchanged. Back up before upgrading. Existing archives remain supported. Shared browser contracts now check exact dashboard styling, widths and access boundaries in both themes and with JavaScript disabled.

## Folkweave 0.4.0

Native owner/admin metrics now show aggregate membership, content, storage and recent activity, with owner-only operational summaries for backups and configured suite connections. The pages work without JavaScript and follow the application's existing permissions.

Rate limiting uses the bounded shared implementations while retaining the app's own budgets. Release preparation, artifact verification, browser fixtures and immutable CI workflows use the published Comfylib pin directly. A release preflight verifies version metadata, cold module caches and the exact shared workflow revision before publication.

No application database migration is added. Existing service configuration and account permissions remain supported.

## Install

Choose a [Linux archive or Debian package](docs/releases.md),
[Docker Compose](docs/containers.md), or the
[Comfyware Gentoo overlay](https://github.com/airencracken/comfyware#add-the-overlay).
Read the [hosting](docs/deployment.md), [backup/recovery](docs/automatic-backups.md),
and [release notes](docs/release-notes.md) before sharing an installation.
The source demos below use disposable local data.

## Try it

Build with Go 1.26.11 or newer:

```sh
make build
./dist/folkweave demo
```

Open <http://127.0.0.1:8084>. Sign in as `demo` with the password
`folkweave-demo-only`. The demo contains fictional people and locally generated
sample illustrations. It uses a temporary data directory that is removed on
shutdown and only binds to a loopback address.

## Start your own gathering

```sh
./dist/folkweave create-owner --data ./data --username owner --name "Your name" --title "Our people"
./dist/folkweave serve --data ./data
```

The owner command prompts for and confirms a password with terminal echo disabled.
It creates the first owner and gathering together, or adds another owner to an
existing gathering without changing its settings or other accounts. Existing
usernames are rejected; use `set-password` for password recovery. The original
`init` command remains available for first-time initialization only.
Automation can supply one line through stdin using `--password-stdin`. Passwords
need at least 12 Unicode characters and at most 72 UTF-8 bytes. Passwords are
hashed with bcrypt; session and invitation secrets are stored only as digests.

For HTTPS behind a reverse proxy:

```sh
./dist/folkweave serve --data ./data --listen 127.0.0.1:8084 \
  --base-url https://folkweave.example.org --trusted-proxies 127.0.0.1/32,::1/128
```

Use the actual external origin for `--base-url`. This controls accepted host
headers, invitation links, and secure cookies. Only trust proxy networks you
control. The Caddy example is in [packaging/Caddyfile](packaging/Caddyfile).
The data directory must be private, mode `0700`; its database must be `0600`.

## What is here

- Invite-only accounts, expiring invitations, and owner-managed membership.
- A chronological timeline, stable cursor pagination, and an explicit new-post button.
- Short or long plain-text posts, clickable web links, and up to four photos.
- Inline photo previews, descriptions, ordering, and draft protection with Alpine.
- Replies that refresh immediately, retry-safe submissions, and private bookmarks.
- Author edits, confirmed deletion, and owner removal of posts and replies.
- Member profiles, sign-out, and host-assisted password recovery.
- Private photo routes, account exports, verified snapshots, and automatic backups.
- Ordinary forms and navigation that work with JavaScript disabled.
- Light and dark themes, keyboard controls, and layouts checked at 320–1280 pixels.

Shared photos are normalized to JPEG, oriented using JPEG EXIF, resized to at
most 1920 pixels on either side, and stripped of metadata. JPEG/PNG inputs are
limited to 5 MiB and 20 megapixels each. **Original uploads are not retained.**
Keep original photographs separately; Imvault remains the home for collections.

The first pass has one installation-wide audience. Managed child accounts and
reviewed child feeds are later work; this build does not provide parental
approval controls. [Product notes](docs/product.md) describe the intended scope.

## Owner administration

Owners have an **Admin** link in the main navigation. It opens gathering settings
and backup status. Invitations remain available under **Account**, and membership
controls are under **People**. Ordinary members cannot open gathering settings.

## Accounts and recovery

Owners create invitations under **Invite someone**. Links expire after seven
days, can admit 1–50 people, and are shown only once. An invited member sees
the gathering's earlier posts and photos. Owners can close invitations and
suspend or restore membership under **People**.

To reset a member's password, run as the account that owns the data directory:

```sh
./dist/folkweave set-password --data ./data --username member
```

This prompts for a new password and revokes that member's existing sessions.

## Backups and exports

The database contains the posts, replies, accounts, and normalized photographs.
The running service uses comfylib to create a verified automatic snapshot daily.
The first automatic attempt is about one minute after startup. The owner page
shows backup status. Keep protected copies on another machine as well.

| Environment variable | Default |
| --- | --- |
| `FOLKWEAVE_DATA_DIR` | `./data` |
| `FOLKWEAVE_LISTEN` | `127.0.0.1:8084` |
| `FOLKWEAVE_BASE_URL` | Listen origin |
| `FOLKWEAVE_TRUSTED_PROXIES` | None |
| `FOLKWEAVE_BACKUP_INTERVAL` | `24h`; `0` disables |
| `FOLKWEAVE_BACKUP_KEEP` | `7` |
| `FOLKWEAVE_BACKUP_MAX_BYTES` | `10737418240` |
| `FOLKWEAVE_BACKUP_COPY_MAX_BYTES` | `5368709120` |

A manual snapshot uses a fresh directory and includes an integrity check:

```sh
./dist/folkweave backup --data ./data --output ./snapshot
./dist/folkweave restore --from ./snapshot/folkweave.db --data ./restored-data
```

Restore refuses an existing target and verifies the copied database before
publishing it. Automatic archives contain `folkweave.db` in their compressed
archive; unpack an archive privately and use that file with `restore`.

Members can download their own posts, photographs, and replies as a ZIP archive
from **Account**. Credentials and other members' posts are excluded.

## Containers

The [Docker Compose guide](docs/containers.md) uses the version-pinned
`ghcr.io/airencracken/folkweave:v0.3.4` image and a private named volume.
Container port 8080 maps to localhost port 8084. It runs as UID/GID 10001.

## Development and checks

```sh
make check
make test-race
make test-mutations
npm ci --prefix scripts/browser
make test-browser
```

Browser checks use `/usr/bin/chromium` when present. Set `CHROMIUM` to another
executable, or install the pinned Playwright browser with
`cd scripts/browser && npx playwright install chromium`.

Tests cover route existence and methods, API contracts, cookie/CSRF policy,
invitation redemption under concurrency, injected transaction failures, stable
pagination during writes, media boundaries and orientation, snapshot/restore,
JavaScript/no-JavaScript flows, composer lifecycle, and mutations of key guards.

[Architecture](docs/architecture.md) · [HTTP API](docs/api.md) · [Mascot](docs/mascot.md)

## Support

You can [support Folkweave on Ko-fi](https://ko-fi.com/airencracken).
Support helps maintain Folkweave and the other [Comfyware projects](https://comfyware.org/).

## License

AGPL-3.0-or-later. See [LICENSE](LICENSE). Embedded HTMX and Alpine releases,
licenses, sources, and checksums are recorded in [THIRD_PARTY.md](THIRD_PARTY.md).

## Common setup

New source builds share [browser and command-line setup](docs/setup.md) with the
other Comfyware apps: `folkweave setup` authorizes the first-owner browser page,
and `folkweave create-owner --username NAME` prompts privately by default.

Optional app navigation, shared sign-in and discussion returns are described in
[Comfyware connections](docs/suite.md).

## Display timezone

In your account settings (`/account`), choose **Times and dates**, enter a city
timezone, and select **Save timezone**. The saved choice follows your account
across devices. UTC is the default. Device detection fills the field when
JavaScript is available; manual selection and saving work without it.
Admin reporting days remain UTC.
