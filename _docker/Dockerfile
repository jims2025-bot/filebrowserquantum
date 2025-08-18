# -------------------------
# Stage 1: Build Go backend
# -------------------------
FROM golang:1.24-alpine AS base
ARG VERSION
ARG REVISION
WORKDIR /app

# Copy go.mod and go.sum first for dependency download
COPY ./backend/go.mod ./backend/go.sum ./
RUN go mod download

# Copy the rest of the backend
COPY ./backend ./

# Optional: fix swagger symlink if needed
RUN ln -s swagger /usr/local/go/src/

# Build the backend binary
RUN go build -ldflags="-w -s \
  -X 'github.com/jims2025-bot/filebrowserquantum/backend/common/version.Version=${VERSION}' \
  -X 'github.com/jims2025-bot/filebrowserquantum/backend/common/version.CommitSHA=${REVISION}'" \
  -o filebrowser .

# -------------------------
# Stage 2: Build frontend
# -------------------------
FROM node:slim AS nbuild
WORKDIR /app

# Copy package.json first for faster npm install
COPY ./frontend/package.json ./
RUN npm install --maxsockets 1

# Copy frontend source
COPY ./frontend/ ./
RUN npm run build-docker

# -------------------------
# Stage 3: Final image
# -------------------------
FROM alpine:latest
RUN apk add --no-cache ca-certificates mailcap tzdata exiftool curl xz && \
    ARCH=$(uname -m) && \
    case "$ARCH" in \
      x86_64)   URL="https://johnvansickle.com/ffmpeg/releases/ffmpeg-release-amd64-static.tar.xz" ;; \
      aarch64)  URL="https://johnvansickle.com/ffmpeg/releases/ffmpeg-release-arm64-static.tar.xz" ;; \
      armv7l)   URL="https://johnvansickle.com/ffmpeg/releases/ffmpeg-release-armhf-static.tar.xz" ;; \
      *)        echo "Unsupported architecture: $ARCH" && exit 1 ;; \
    esac && \
    mkdir -p /tmp/ffmpeg && \
    curl -L "$URL" | tar -xJ -C /tmp/ffmpeg && \
    mv /tmp/ffmpeg/ffmpeg-*/ffmpeg /usr/local/bin/ && \
    mv /tmp/ffmpeg/ffmpeg-*/ffprobe /usr/local/bin/ && \
    chmod +x /usr/local/bin/ffmpeg /usr/local/bin/ffprobe && \
    rm -rf /tmp/ffmpeg /var/cache/apk/*

ENV FILEBROWSER_FFMPEG_PATH="/usr/local/bin/"
ENV FILEBROWSER_NO_EMBEDED="true"

# Add user
RUN adduser -D -s /bin/true -u 1000 filebrowser
USER filebrowser
WORKDIR /home/filebrowser

# Copy backend binary
COPY --from=base --chown=filebrowser:1000 /app/filebrowser* ./

# Copy config
COPY --from=base --chown=filebrowser:1000 /app/config.media.yaml ./config.yaml

# Copy frontend dist
COPY --from=nbuild --chown=filebrowser:1000 /app/dist/ ./http/dist/

# Sanity checks
RUN [ "filebrowser", "version" ]
RUN [ "ffmpeg", "-version" ]
RUN [ "ffprobe", "-version" ]
RUN [ "exiftool", "-ver" ]

USER root
EXPOSE 80
ENTRYPOINT [ "./filebrowser" ]
