# Folkweave

<img src="internal/folkweave/static/loom.webp" alt="Folkweave’s little wooden loom" width="144" height="144" align="right">

A private timeline for your people. A thought, a photograph, a bit of your day.

Folkweave is a small, self-hosted Comfyware app for friends, family, or any
group who choose each other. Each installation has its own accounts and
invite-only membership. Members can see everything the group has posted.

Current release: **[0.7.0](https://github.com/airencracken/folkweave/releases/tag/v0.7.0)**. [What's new](docs/release-notes.md)

<br clear="right">

[Comfyware](https://comfyware.org/) · [Project page](https://comfyware.org/folkweave/) · [Help and bugs](https://github.com/airencracken/folkweave/issues)

## Try it

Build with Go 1.26.11 or later (or 1.27.4 or later on the 1.27 series), then
start the demo:

```sh
make build
./dist/folkweave demo
```

Open <http://127.0.0.1:8084> and sign in as `demo` with the password
`folkweave-demo-only`. The demo has fictional people and photos. It listens
only on loopback and deletes its data when it stops.

## Install

Install a [Linux archive or Debian package](docs/releases.md), run it with
[Docker Compose](docs/containers.md), or use the
[Comfyware Gentoo overlay](https://github.com/airencracken/comfyware#add-the-overlay).

Create the first owner in the browser with `folkweave setup`, or in the terminal:

```sh
./dist/folkweave create-owner --data-dir ./data --username owner --name "Your name" --title "Our people"
./dist/folkweave serve --data-dir ./data
```

`create-owner` asks for the password twice without echoing it. Run it again to
add another owner; it leaves the group's settings alone. [Setup](docs/setup.md)
covers both paths, and [Comfyware connections](docs/suite.md) covers the
optional Apps menu and community sign-in.

Behind an HTTPS reverse proxy, give the public origin and the proxy's address:

```sh
./dist/folkweave serve --data-dir ./data --listen 127.0.0.1:8084 \
  --base-url https://folkweave.example.org --trusted-proxies 127.0.0.1/32,::1/128
```

Read [hosting](docs/deployment.md) and [backups](docs/automatic-backups.md)
before you invite anyone.

## What members can do

- Join with an invitation link and see the group's earlier posts.
- Post short or long text, links, and up to four photos with descriptions.
- Reply, save posts for later, and edit or remove their own words.
- Pick a display timezone under **Account**, in **Times and dates**.
- Choose System, Light, or Dark, and a color palette, from the **Account** menu.
- Download their own posts, photos, and replies from **Account**.

The timeline is newest first and has an end. Posting, replying, and every
other form work without JavaScript.

Folkweave resizes shared photos to at most 1920 pixels, converts them to JPEG,
and strips their metadata. Each upload can be up to 5 MiB and 20 megapixels.
**Folkweave does not keep your originals**, so keep them somewhere else, such
as Imvault.

Everyone in an installation sees the same timeline. Parent-managed accounts
and reviewed feeds are not built. [Product notes](docs/product.md) describe
the scope.

## Running the group

Owners see **Admin** in the main navigation. It holds three pages:

- **Settings**: the group's name, description, site colors, and backup status.
- **Invitations**: create links that admit 1 to 50 people for seven days. Each
  link is shown once.
- **Metrics**: health totals for the installation.

Suspend or restore a member under **People**. Suspending ends their sessions
and keeps their posts.

To reset a password, run this as the account that owns the data directory. It
ends that member's sessions:

```sh
./dist/folkweave set-password --data-dir ./data --username member
```

## What Folkweave records

Metrics shows totals that Folkweave counts from the data it already holds:
members, suspended members, open invitations, posts, replies, photos, photo
storage, new posts in the past seven days, and database size. It also shows
the running version, backup status, and which optional connections are set up.

For site health, Folkweave counts requests per day for each kind of page, such
as `GET /posts/{id}`: how many, how many failed, how fast, and how much data
was sent. It also counts failed sign-ins, rate-limited attempts, and rejected
invitation links. These counts are kept for 90 days.

Folkweave does not record who signed in, who opened which post or photo, IP
addresses, or browsers. Upgrading deletes the sign-in and view records that
0.5.0 and 0.6.0 kept.

## Backups and exports

The server makes a verified backup every day under the data directory, and you
can make one by hand. [Backups and recovery](docs/automatic-backups.md) has the
commands and settings. Copy backups to another machine too. A member's export
holds only their own posts, photos, and replies.

## Development

```sh
make check
make test-race
make test-mutations
npm ci --prefix scripts/browser
make test-browser
```

Browser tests use `/usr/bin/chromium` when it exists. Set `CHROMIUM` to use a
different browser, or install Playwright's with
`cd scripts/browser && npx playwright install chromium`.

[Architecture](docs/architecture.md) · [HTTP API](docs/api.md) · [Mascot](docs/mascot.md)

## Support

You can [support Folkweave on Ko-fi](https://ko-fi.com/airencracken). It helps
maintain Folkweave and the other [Comfyware projects](https://comfyware.org/).

## License

AGPL-3.0-or-later. See [LICENSE](LICENSE). [THIRD_PARTY.md](THIRD_PARTY.md)
lists the bundled HTMX and Alpine releases with their licenses and checksums.
