# Run Folkweave with Docker Compose

Images are on [GHCR](https://github.com/airencracken/folkweave/pkgs/container/folkweave)
as `ghcr.io/airencracken/folkweave:v0.6.0`, for linux/amd64 and linux/arm64.
There is no `latest` tag, and published tags never change.

Download `docker-compose.yml` from the release you want. Put the public origin
in a private `.env` file next to it:

```sh
FOLKWEAVE_BASE_URL="https://folkweave.example.org"
```

Create the first owner and start the service. The owner command asks for the
password and does not need the server running:

```sh
docker compose pull
docker compose run --rm folkweave create-owner --username owner --name "Your name" --title "Our group"
docker compose up -d
```

Compose publishes container port 8080 on `127.0.0.1:8084`. Put your HTTPS
reverse proxy in front of it.

The container runs as UID and GID 10001 with a read-only root filesystem, no
added capabilities, and `no-new-privileges`. Data lives in the named volume at
`/data`. If you bind-mount a directory instead, make it private (mode 0700) and
owned by 10001:10001. Do not run the container as root.

The health check calls `/healthz`. Settings are in [hosting](deployment.md) and
backups in [backups and recovery](automatic-backups.md).

## Upgrade

Back up first. Change the image tag in `docker-compose.yml`, then run
`docker compose pull` and `docker compose up -d`. The named volume survives
container removal; `docker compose down --volumes` deletes it.

To build from source instead:
`docker compose -f docker-compose.yml -f docker-compose.build.yml up --build`.
