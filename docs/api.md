# HTTP API

The first pass provides a versioned, session-authenticated read API. Sign in
through the normal browser flow. Every API response is private and `no-store`.
Unauthenticated requests return 401. Errors use a plain-text response.

| Method | Route | Result |
| --- | --- | --- |
| GET | `/api/v1/me` | Current member profile |
| GET | `/api/v1/posts` | Newest posts; up to 20 |
| GET | `/api/v1/posts?before=CURSOR` | Next anchored page |
| GET | `/api/v1/posts/ID` | A post and up to 500 chronological replies |
| GET | `/photos/ID` | Session-protected normalized JPEG |
| GET | `/account/export` | Current member's ZIP archive |

The list response has exactly `posts` (array), `next` (opaque string, empty at
the end), and `anchor` (nonnegative integer). Do not construct cursor values.

Post fields: `id`, `author`, `body`, `created_at`, `edited`, `revision`,
`photos`, `reply_count`, and `saved`. The detail response may additionally
include `replies`. Timestamps are RFC3339. A photo has `id`, `alt`, `width`, and
`height`; fetch it through `/photos/ID`. An author has `id`, `username`, `name`,
`bio`, and `role`. A reply has `id`, `author`, `body`, and `created_at`.

`saved` is private to the requesting member. Password hashes, session secrets,
invitation tokens, submission keys, and photo bytes are never in these JSON
responses. Account exports contain only that member's own posts, photographs,
and replies; they are not a community archive.

Browser forms own writes in this pass. They require CSRF tokens and current
membership. Post and reply forms carry a unique submission key, and retries
with that key and the original content resolve to the existing write.
