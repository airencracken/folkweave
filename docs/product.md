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

Members can repost someone else's post or reply once to the same timeline.
The repost credits the author and links to the original. Edits are reflected
in the repost. If the source is removed, the repost says so and keeps its
own discussion.

Open a photo to see its description and browse the other photos in the post.
Search looks for literal text in posts, replies, and photo descriptions. Each
result opens its source, and subsequent searches reflect edits and removals.

Saved posts are personal. Profiles show who is speaking. Owners can close
invitations, suspend members, and remove posts and replies. Authors can edit
their posts and delete them from the **More** menu. Authors can also remove
their own replies. Suspending a member ends their sessions.

Catch up shows posts after a member’s bookmark, oldest first. A visit has a
fixed endpoint, and members can save their place after each page. Opening a
post leaves the bookmark alone. The Replies inbox includes responses to members’
own posts and replies added after they follow a conversation. Members can mark
the inbox seen and unfollow conversations from the post. Bookmarks, inbox
state, and follows belong to each member.

Photos are stored in the SQLite database. Folkweave runs on its own, without
Imvault, Witmoot, or Songstead.

## What the owner can see

The Metrics page is for checking that the installation is healthy. It shows
totals counted from the data Folkweave already holds when the page opens:
members, suspended members, open invitations, posts, replies, photos, photo
storage, new posts in the past seven days, and database size. It also shows the
running version, backup status, and which optional connections are set up.

Below that, Site health shows daily requests for each kind of page, error
rates, response times, and data sent, and Security shows failed sign-ins,
rate-limited attempts, and rejected invitation links. These counts are kept
for 90 days and name no one.

Folkweave does not record who signed in, who opened which post or photo, IP
addresses, or browsers, and it never ranks members or posts.

## Family controls

Not built. Parent-managed accounts could control joining and outgoing posts.
Reviewed viewing would need approval of every visible post, reply, photo, and
edit. Until both exist, do not describe the ordinary member view as a reviewed
or parent-approved feed.

## Attention and ownership

The timeline has an end. Folkweave is not federated and has no followers,
ranking, or engagement targets. Members choose when to visit. Members
can export their own contributions, and the server makes verified backups.

Possible next steps include more account self-service, reporting posts,
reviewed audiences, and links to the other Comfyware apps.
Each needs its own decisions about permissions and lifecycle.
