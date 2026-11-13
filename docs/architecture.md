# Architecture

One Go process serves the pages, styles, and bundled JavaScript. SQLite holds
accounts, sessions, invitations, posts, replies, photos, and saved posts. The
server uses one database connection and locks its data directory so a second
server cannot open it. Multi-row writes run in transactions.

## Browser

HTMX submits ordinary forms and swaps in server-rendered fragments. Alpine's
CSP build runs the composer: selected files, previews, descriptions, order,
and the unsaved-changes prompt. All scripts are local files, and the Content
Security Policy allows no inline scripts or `eval`.

The composer stays outside timeline swaps. A failed post replaces only the
error message and keeps the text and files. A successful post returns a fresh
composer and updates the timeline. A reply returns the whole discussion.
Repeated clicks send one request, and each form carries a submission key so a
retry after a lost response does not post twice. Keys outlive deleted posts,
so a late retry cannot bring one back.

Without JavaScript, links and POST-redirect-GET forms do the same work. A
browser cannot keep a file selection after a full-page error, so the page asks
the member to choose the photos again and keeps their text.

`theme.js` runs before the stylesheets and sets `data-theme` from browser
storage, so a saved theme shows on first paint.

## Membership and privacy

Every content, profile, API, export, and photo route needs a current session.
The server checks the session on each request, so suspension takes effect at
once, and writes recheck membership inside their transaction. Photos have no
public file path. Deleting a post deletes its photos, replies, and saves.

CSRF tokens come from the session, or from a private form cookie before sign-in.
Every POST needs the token and passes Go's cross-origin check. Sign-in attempts
are rate limited per network. Private pages send `no-store` and
`no-referrer`, and HTMX keeps no history cache.

## Storage

Pages of the timeline are anchored to post IDs, so new posts do not shift older
pages, and edits and replies do not move a post.

Uploads are size-checked before decoding, then resized, rotated, and saved in
the same transaction as their post. At most two uploads are processed at once.
Folkweave keeps only the resized copy without metadata. Backups use SQLite
`VACUUM INTO` with a size cap and an integrity check.

Comfylib provides invitation tokens and status, shared account and invitation
styles, profile checks, password prompts, client address handling, and backup
compression and retention. Folkweave owns membership rules, routes, the schema,
and everything social.
