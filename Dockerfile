FROM golang:1.23.0-bookworm as builder
ARG GIT_SHA="<not provided>"


RUN apt-get update && apt-get install -y --no-install-recommends --no-install-suggests wget
RUN wget -q -d https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp_linux -O /usr/local/bin/yt-dlp \
 && chmod +x /usr/local/bin/yt-dlp

ADD *.go /src/.
ADD config /src/config
ADD database /src/database
Add ffmpeg /src/ffmpeg
ADD handlers /src/handlers
ADD media /src/media
ADD originals /src/originals
Add playlists /src/playlists
Add ytdlp /src/ytdlp
ADD go.mod /src/.

RUN cd /src && go mod tidy
RUN cd /src && go build -ldflags "-X main.GitSHA=${GIT_SHA} -X main.BuildDate=$(date +%Y-%m-%d)" -o server *.go

FROM debian:bookworm-slim

RUN apt-get update \
 && apt-get install -y --no-install-recommends --no-install-suggests \
   ffmpeg \
 && rm -rf /var/lib/apt/lists/*

COPY --from=0 /usr/local/bin/yt-dlp /usr/local/bin/yt-dlp 
COPY --from=0 /src/server /opt/server
ADD templates /opt/templates
ADD static /opt/static

WORKDIR /opt
CMD ["/opt/server"]
