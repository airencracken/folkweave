# Folkweave

A private timeline for a group of people who choose each other: a family,
friends, or another small community. A short thought, a longer update, photos,
and links all fit in one post.

Each installation is its own place. The owner invites members and explains who
is in the group and what the house rules are. New members can see the group's
earlier posts, and the invitation and join pages say so before they join.

## What is built

Members join through an invitation, choose a name, post text or photos, reply,
and come back to earlier posts. Each post has a stable address. The timeline is
newest first. Replies and edits keep a post in place. **Older posts** loads
the next page. New posts wait behind a button so the page does not jump.

Saved posts are personal. Profiles show who is speaking. Owners can close
invitations, suspend members, and remove posts and replies. Authors can edit
and remove their own. Suspending a member ends their sessions.

Photos are stored in the SQLite database. Folkweave runs on its own, without
Imvault, Witmoot, or Songstead.

## What the owner can see

The Metrics page is for checking that the installation is healthy. It shows
totals counted from the data Folkweave already holds when the page opens:
members, suspended members, open invitations, posts, replies, photos, photo
storage, new posts in the past seven days, and database size. It also shows the
running version, backup status, and which optional connections are set up.

Folkweave does not record sign-ins, page views, or who opened which post or
photo, and it never ranks members or posts. The server log holds errors and
rejected requests.

## Family controls

Not built. Parent-managed accounts could control joining and outgoing posts.
Reviewed viewing would need approval of every visible post, reply, photo, and
edit. Until both exist, do not describe the ordinary member view as a reviewed
or parent-approved feed.

## Attention and ownership

The timeline has an end. Folkweave is not federated and has no followers,
boosts, ranking, or engagement targets. Members choose when to visit. Members
can export their own contributions, and the server makes verified backups.

Possible next steps: a saved reading position, more account self-service,
reporting posts, reviewed audiences, and links to the other Comfyware apps.
Each needs its own decisions about permissions and lifecycle.
