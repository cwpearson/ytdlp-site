# ytdlp-site

```
go mod tidy
```

```bash
export YTDLP_SITE_ADMIN_INITIAL_PASSWORD=abc123
export YTDLP_SITE_SESSION_AUTH_KEY=v9qpt37hc4qpmhf
go run *.go
```

## Environment Variables

* `YTDLP_SITE_ADMIN_INITIAL_PASSWORD`: password of the `admin` account, if the account does not exist
* `YTDLP_SITE_SESSION_AUTH_KEY`: admin-selected secret key for the cookie store

## Docker

```bash
docker build -t server .

docker run --rm -it \
  -p 3000:8080 \
  --env YTDLP_SITE_ADMIN_INITIAL_PASSWORD=abc123 \
  server

docker run --rm -it \
  -p 3000:8080 \
  --env YTDLP_SITE_DOWNLOAD_DIR=/downloads \
  --env YTDLP_SITE_CONFIG_DIR=/config \
  --env YTDLP_SITE_ADMIN_INITIAL_PASSWORD=abc123 \
  -v $(realpath downloads):/downloads \
  server
```