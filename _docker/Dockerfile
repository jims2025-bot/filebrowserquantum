# -------------------------
# Stage 1: Build Go backend
# -------------------------
FROM golang:1.25-alpine AS base
ARG VERSION
ARG REVISION
WORKDIR /app

# Copy go.mod and go.sum from repo root
COPY ./go.mod ./go.sum ./
RUN go mod download

# Copy backend source
COPY ./backend ./backend

# Build the backend binary
WORKDIR /app/backend
RUN go build -ldflags="-w -s \
  -X 'github.com/jims2025-bot/filebrowserquantum/backend/common/version.Version=${VERSION}' \
  -X 'github.com/jims2025-bot/filebrowserquantum/backend/common/version.CommitSHA=${REVISION}'" \
  -o ../filebrowser

# -------------------------
# Stage 2: Build frontend
# -------------------------
FROM node:slim AS nbuild
WORKDIR /app

# Install frontend deps
COPY ./frontend/package.json ./
RUN npm install --maxsockets 1

# Build frontend
COPY ./frontend/ ./
RUN npm run build-docker

# -------------------------
# Stage 3: Final image
# -------------------------
FROM python:3.11-slim

# Install system dependencies (including exiftool and build tools for dlib)
RUN apt-get update && apt-get install -y --no-install-recommends \
  ca-certificates \
  tzdata \
  exiftool \
  curl \
  xz-utils \
  build-essential \
  cmake \
  libopenblas-dev \
  liblapack-dev \
  libx11-dev \
  libgtk-3-dev \
  python3-dev \
  && rm -rf /var/lib/apt/lists/*

# Install ffmpeg tools safely
RUN ARCH=$(uname -m) && \
  case "$ARCH" in \
  x86_64)   URL="https://johnvansickle.com/ffmpeg/releases/ffmpeg-release-amd64-static.tar.xz" ;; \
  aarch64)  URL="https://johnvansickle.com/ffmpeg/releases/ffmpeg-release-arm64-static.tar.xz" ;; \
  armv7l)   URL="https://johnvansickle.com/ffmpeg/releases/ffmpeg-release-armhf-static.tar.xz" ;; \
  *)        echo "Unsupported architecture: $ARCH" && exit 1 ;; \
  esac && \
  mkdir -p /tmp/ffmpeg && \
  curl --retry 5 --retry-delay 3 -L "$URL" -o /tmp/ffmpeg/ffmpeg.tar.xz && \
  tar -xJf /tmp/ffmpeg/ffmpeg.tar.xz -C /tmp/ffmpeg && \
  mv /tmp/ffmpeg/ffmpeg-*/ffmpeg /usr/local/bin/ && \
  mv /tmp/ffmpeg/ffmpeg-*/ffprobe /usr/local/bin/ && \
  chmod +x /usr/local/bin/ffmpeg /usr/local/bin/ffprobe && \
  rm -rf /tmp/ffmpeg

# Install Python ML dependencies
RUN pip install --no-cache-dir face_recognition opencv-python dlib

ENV FILEBROWSER_FFMPEG_PATH="/usr/local/bin/"
ENV FILEBROWSER_NO_EMBEDED="true"

# Add non-root user
RUN useradd -ms /bin/bash -u 1000 filebrowser
USER filebrowser
WORKDIR /home/filebrowser

# Copy backend binary
COPY --from=base --chown=filebrowser:1000 /app/filebrowser* ./

# Copy config
COPY --from=base --chown=filebrowser:1000 /app/backend/config.media.yaml ./config.yaml

# Copy frontend dist
COPY --from=nbuild --chown=filebrowser:1000 /app/dist/ ./http/dist/

# Copy Facial Recognition Python Script & Data Models
COPY --chown=filebrowser:1000 backend/facerec.py ./
COPY --chown=filebrowser:1000 backend/shape_predictor_68_face_landmarks.dat ./
COPY --chown=filebrowser:1000 backend/dlib_face_recognition_resnet_model_v1.dat ./

# Sanity checks
RUN ["./filebrowser", "version"]
RUN ["ffmpeg", "-version"]
RUN ["ffprobe", "-version"]
RUN ["exiftool", "-ver"]
RUN ["python", "-c", "import face_recognition; import cv2; import dlib"]

# Switch to root to expose and run
USER root
EXPOSE 80
ENTRYPOINT [ "./filebrowser" ]
