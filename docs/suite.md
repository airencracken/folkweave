# Comfyware connections

Each Comfyware app keeps its own accounts, sessions, and data. These
connections are optional and off by default. Set them in your service
configuration or the Compose `.env` file, then restart.

## Apps menu

Set `FOLKWEAVE_APPS` to a JSON list of app IDs and their public addresses. The
menu leaves out the current app. For example:

```sh
FOLKWEAVE_APPS='[{"id":"imvault","url":"https://photos.example.org"},{"id":"songstead","url":"https://music.example.org"},{"id":"witmoot","url":"https://discuss.example.org"},{"id":"folkweave","url":"https://people.example.org"},{"id":"tidings","url":"https://read.example.org"}]'
```

Folkweave refuses to start with unknown or repeated IDs, or addresses with
credentials, queries, or fragments. It never contacts these addresses, and the
menu gives no access to the other apps.

## Community sign-in

Register Folkweave with an OpenID Connect provider, using the callback
`https://your-folkweave.example.org/auth/shared/callback` (your
`FOLKWEAVE_BASE_URL` followed by `/auth/shared/callback`). Then set:

```sh
FOLKWEAVE_SHARED_ISSUER="https://identity.example.org"
FOLKWEAVE_SHARED_CLIENT_ID="folkweave"
FOLKWEAVE_SHARED_CLIENT_SECRET="keep-this-in-your-protected-service-configuration"
```

The issuer must use HTTPS, except a loopback address for testing. Give each app
its own client registration, and protect the secret.

To link your identity, sign in with your password, open the **Account** menu,
and choose **Community sign-in**. After that, the sign-in page offers it.
Folkweave never creates an account from an identity or links one by email.
Invitations and suspensions still apply. Unlinking leaves your password
working, and signing out of Folkweave does not sign you out elsewhere. Keep a
working owner password for when the provider is down.

Linked identities are stored in the database and included in backups. A
sign-in in progress expires after five minutes or when the server restarts.
