FROM golang:1.23.0-bookworm as builder

RUN apt-get update && apt-get install -y --no-install-recommends --no-install-suggests wget
RUN wget https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp_linux -O /usr/local/bin/yt-dlp \
 && chmod +x /usr/local/bin/yt-dlp

ADD *.go /src/.
ADD go.mod /src

RUN cd /src && go mod tidy
RUN cd /src && go build -o server *.go

FROM debian:bookworm-slim

RUN apt-get update \
 && apt-get install -y --no-install-recommends --no-install-suggests \
   ffmpeg \
 && rm -rf /var/lib/apt/lists/*

COPY --from=0 /usr/local/bin/yt-dlp /usr/local/bin/yt-dlp 
COPY --from=0 /src/server /opt/server
ADD templates /opt/templates

WORKDIR /opt
CMD ["/opt/server"]
