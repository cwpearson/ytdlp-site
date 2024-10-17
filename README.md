# ytdlp-site

```
go mod tidy
```

```bash
export YTDLP_SITE_ADMIN_INITIAL_PASSWORD=abc123
export YTDLP_SITE_SESSION_AUTH_KEY=v9qpt37hc4qpmhf
go run \
 -ldflags "-X ytdlp-site/config.gitSHA=$(git rev-parse HEAD) -X ytdlp-site/config.buildDate=$(date +%Y-%m-%d)" \
 *.go
```

## Environment Variables

* `YTDLP_SITE_ADMIN_INITIAL_PASSWORD`: password of the `admin` account, if the account does not exist
* `YTDLP_SITE_SESSION_AUTH_KEY`: admin-selected secret key for the cookie store
* `YTDLP_SITE_SECURE`: set to `ON` for HTTPS deployments

## Docker

```bash
docker build --build-arg GIT_SHA=$(git rev-parse HEAD) \
  -t server .

docker run --rm -it \
  -p 3000:8080 \
  --env YTDLP_SITE_ADMIN_INITIAL_PASSWORD=abc123 \
  --env YTDLP_SITE_SESSION_AUTH_KEY=avowt7n8 \
  server

docker run --rm -it \
  -p 3000:8080 \
  --env YTDLP_SITE_DATA_DIR=/data \
  --env YTDLP_SITE_CONFIG_DIR=/config \
  --env YTDLP_SITE_ADMIN_INITIAL_PASSWORD=abc123 \
  -v $(realpath data):/data \
  server
```

## GHCR Deploy

Build and push this container to ghcr

* Create a "personal access token (classic)" with write:packages
  * account > settings > developer settings > personal access tokens > tokens (classic) > generate new token (classic)
* Put that personal access token as the repository actions secret `GHCR_TOKEN`.

## Roadmap

- [ ] edit original metadata
- [ ] Download playlists
- [ ] change from Audio -> Video
- [x] Provide a better name for downloaded files
- [x] Environment variable to control whether "Secure" flag set on cookie
- [x] Allow custom FPS for video transcode
- [ ] Provide an about page
  - `ffmpeg` version
  - `yt-dlp` version
  - disk space
- [ ] skip buttons for audio player
- [ ] Track progress via video URL rather than ID
- [ ] Show author on video page
  - [ ] Link to author on video page
- [ ] Link to author on videos page
- [ ] video clips
- [ ] move original video to bottom
- [x] sort videos most to least recent
- [x] header on playlist page
