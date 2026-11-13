# Folkweave release notes

Back up the stopped data directory before every upgrade, and keep that copy
until you are sure you will not go back. An older Folkweave cannot open a
database that a newer one has upgraded.

## 0.7.1

- Photo uploads accept up to 5 MiB again, and sizes are shown in MiB.

### For operators

- Built with Comfylib 0.7.1.

## 0.7.0

- Owners reach everything from **Admin**: Settings, Invitations, and Metrics.
  The **Account** menu holds your account, the theme, colors, and Sign out.
- Choose System, Light, or Dark, and a color palette, from the **Account**
  menu. Owners set the site colors under **Admin**, in Settings. Sage is the
  default.
- **Times and dates** has its own panel on the Account page.
- Posts no longer repeat "Everyone here" under each one.
- Error pages and messages use plain words.
- The invitation link has a Copy link button.
- Metrics shows site health: requests, errors, response times, and data sent
  for each kind of page, plus failed sign-ins, rate-limited attempts, and
  rejected invitation links. Folkweave no longer records who signed in or who
  opened which post or photo.
- Admin pages use Folkweave's colors, type, and corners.

### For operators

- Startup deletes the sign-in and view records kept by 0.5.0 and 0.6.0, adds
  tables for site health counts, and adds a site palette setting. Back up first.
- Photo uploads are limited to 5,000,000 bytes, down from 5 MiB (5,242,880).
- API errors are JSON with `status` and `error`, instead of plain text.
- Owners see a short backup summary. The full error is in the server log.
- Help lists `--data-dir`. `--data` and `init` still work but are no longer
  listed.
- Built with Comfylib 0.7.0.

## 0.6.0

- Pick a display timezone under **Account**, in **Times and dates**. Search for
  a city or enter a name such as `America/Los_Angeles`, then select **Save
  timezone**. UTC is the default.
- Your choice follows your account to every device.
- With JavaScript on, **Use this device's timezone** fills in the field for
  you. Saving is always your choice.
- Dates on every page follow your choice.

### For operators

- Startup adds a table for display preferences.
- Stored timestamps and API responses stay in UTC.

## 0.5.0

- The owner Metrics page uses the same layout as the other Comfyware apps.
- This release recorded sign-ins and post and photo opens for owners to see.
  The next release removes these records.

### For operators

- Startup adds tables for the sign-in and view records.

## 0.4.0

- Owners get a Metrics page with member, content, and storage totals, backup
  status, and the optional connections that are set up. It works without
  JavaScript.

### For operators

- Sign-in rate limits use shared Comfylib code and keep the same limits.
- No database changes.

## 0.3.4

- Owners see **Admin** in the main navigation.

### For operators

- Release checks run on fresh CI runners.
- No database changes.

## 0.3.3

### For operators

- Source builds report the right version. 0.3.2 built from source reported 0.3.1.

## 0.3.2

### For operators

- The Apps menu, community sign-in, and shared styles come from published
  Comfylib 0.3.0 and its OIDC module 0.1.0.
- Release, container, and configuration checks run from the pinned Comfylib.
- No database changes.

## 0.3.1

### For operators

- OpenRC and Docker Compose pass the Apps menu and community sign-in settings
  through to Folkweave.

## 0.3.0

- An optional Apps menu links to your other Comfyware apps.
- Optional community sign-in with OpenID Connect. Link it from **Account**
  after signing in with your password. Folkweave never creates or links an
  account by matching an email address.
- Photos load faster: the timeline uses cached 320, 640, and 960 pixel copies.

### For operators

- Source builds need Go 1.26.11 or later. golang.org/x/crypto is updated to
  v0.59.0.
- Startup adds a table for cached photo copies. Community sign-in adds a table
  for linked identities when you set it up.
- See [Comfyware connections](suite.md) for settings.

## 0.2.1

### For operators

- Service configuration files use `KEY="value"` for every setting. Existing
  files keep working.

## 0.2.0

- Create the first owner in the browser with `folkweave setup`.
- `create-owner` and `set-password` ask for the password twice without echoing
  it, and accept `--password-stdin`, `--password-file`, and `--data-dir`.
  Changing a password signs that account out everywhere.

## 0.1.1

- `folkweave create-owner --username NAME` creates the first owner or adds
  another one without changing the group's settings.

### For operators

- OpenRC passes plain `KEY=value` settings to the service without `export`.

## 0.1.0

The first public release.

- Invite-only accounts, a newest-first timeline, posts with up to four photos,
  replies, profiles, and saved posts.
- Resized, private photos. Folkweave does not keep originals.
- Account exports and owner removal of posts and replies.
- Daily verified backups, restore, and password recovery.
- Linux amd64 and arm64 archives, Debian packages, container images, and a
  Gentoo overlay package.
