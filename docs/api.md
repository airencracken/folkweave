# HTTP API

A read-only JSON API for signed-in members. Sign in through the browser first;
the API uses the same session. Requests without a session get 401. Errors are
JSON with `status` and `error`, and no response is cached.

| Method | Route | Result |
| --- | --- | --- |
| GET | `/api/v1/me` | Your member profile |
| GET | `/api/v1/posts` | Up to 20 newest posts |
| GET | `/api/v1/posts?before=CURSOR` | The next page |
| GET | `/api/v1/posts/ID` | A post and up to 500 replies, oldest first |
| GET | `/photos/ID` | A photo as JPEG |
| GET | `/account/export` | Your ZIP archive |

A list response has `posts` (array), `next` (cursor string, empty on the last
page), and `anchor` (integer). Use `next` as given; do not build cursors.

A post has `id`, `author`, `body`, `created_at`, `edited`, `revision`,
`photos`, `reply_count`, and `saved`. The single-post response adds `replies`.
Timestamps are RFC 3339 in UTC. A photo has `id`, `alt`, `width`, and `height`;
fetch its bytes from `/photos/ID`. An author has `id`, `username`, `name`,
`bio`, and `role`. A reply has `id`, `author`, `body`, and `created_at`.

`saved` shows only your own saved state. Responses never include password
hashes, session or invitation secrets, or photo bytes. An export holds only
your own posts, photos, and replies.

Writes go through the browser forms.
