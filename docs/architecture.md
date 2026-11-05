# Architecture

One Go process serves embedded templates, styles, and pinned JavaScript. SQLite
stores accounts, sessions, invitations, posts, replies, photographs, and personal
bookmarks. The service has a single database connection and a process lock for
its data directory. Transactions own multi-row writes and invitation redemption.

## Browser responsibilities

HTMX submits ordinary forms and replaces server-rendered fragments. Alpine's CSP
build owns local composer state: selected files, preview URLs, descriptions,
ordering, and unsaved-change prompts. Application components live in external
JavaScript, and all browser assets are served locally. CSP permits neither
`unsafe-eval` nor inline scripts.

The composer remains outside timeline swaps. A failed submission replaces only
its error message, preserving text and selected files. A successful submission
returns a fresh composer and an out-of-band timeline update. A reply returns
the complete updated discussion. Synchronization drops repeated in-flight
submits; persistent submission keys prevent a duplicate write when a response
is lost. Receipts remain after content removal to prevent stale retries from
recreating deleted posts or replies.

Ordinary links and POST/redirect/GET provide the JavaScript-disabled path.
Native forms cannot retain a file selection after a full error response; the
error asks the member to reselect files while retaining their text.

## Membership and privacy

All content, profile, API, export, and media routes require a current session.
Sessions are looked up on every request and immediately fail after suspension
or revocation. Writes also recheck membership inside their transaction.
The initial installation has one invited audience. Photograph bytes never have
a public static path. Foreign keys remove photographs, replies, and bookmarks
with a deleted post.

CSRF tokens are derived from a session or an anonymous HttpOnly form cookie
using comfylib. Mutating requests require the body token and pass Go's
cross-origin protection. Authentication has a bounded per-network attempt
limiter, with trusted proxy handling delegated to comfylib. Private responses
are `no-store`, have a no-referrer policy, and disable HTMX local history caches.

## History and storage

Timeline pagination uses a descending immutable post ID and a fixed page
anchor. Cursor values are validated and bound as SQL parameters. Concurrent
inserts do not shift older pages; edits and replies do not move a post.

Photos are bounded before decode, normalized and oriented in memory, and
inserted in the same transaction as their post. Only two multipart upload
requests may parse or normalize concurrently. Shared copies discard metadata
and originals. Filesystem snapshots use a consistent SQLite `VACUUM INTO`
while retaining the database connection, enforce a byte cap, and run an
integrity check. comfylib supplies automatic archive compression and retention.

comfylib also supplies invitation status, tokens, shared account/invitation
styles, profile validation, password prompting, and trusted client addresses.
Folkweave owns membership policy, routes, schemas, and social behavior.
