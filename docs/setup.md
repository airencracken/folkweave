# Set up Folkweave

Set the public HTTPS origin and listen address first; see
[hosting](deployment.md). Then create the first owner in the browser or the
terminal. With `sudo`, an installed Folkweave finds its data directory and
switches to the service account. On a portable install, run these commands as
the account that owns the data directory.

## Browser

```sh
sudo folkweave setup
```

This prints the setup address and a private key. Start the service, open
`/setup`, and enter the key with your username and password. A browser on the
same host can use the plain HTTP loopback address; from anywhere else, use
HTTPS. The key never appears in a URL or the server log. The setup page closes
once any owner exists.

## Terminal

```sh
sudo folkweave create-owner --username yourname
sudo folkweave set-password --username yourname
```

Both ask for the password twice without echoing it. For automation, use
`--password-stdin` or `--password-file /private/path`; the file must be private.
Pass `--data-dir DIRECTORY` to pick a portable install. Passwords need at least
12 characters. `create-owner` never changes an existing account, and
`set-password` signs that account out everywhere.

`folkweave help COMMAND` shows each command's options. After you sign in,
the group settings and invitations are under **Admin**.
