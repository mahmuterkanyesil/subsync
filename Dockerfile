# Stage 1 — Build
# $BUILDPLATFORM = runner'ın native platformu (her zaman amd64 on GitHub Actions)
# Go cross-compilation ile $TARGETARCH için binary üretir — QEMU gereksiz
FROM --platform=$BUILDPLATFORM golang:1.25rc3-alpine AS builder

ARG TARGETOS=linux
ARG TARGETARCH=amd64

ENV GOTOOLCHAIN=auto
ENV CGO_ENABLED=0
ENV GOOS=$TARGETOS
ENV GOARCH=$TARGETARCH

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o bin/agent ./cmd/agent && \
    go build -o bin/worker ./cmd/worker && \
    go build -o bin/embedder ./cmd/embedder && \
    go build -o bin/api ./cmd/api

# Stage 2 — Run
FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ffmpeg ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /app/bin/ ./bin/

EXPOSE 8080
