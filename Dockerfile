# Stage 1 — Build
FROM golang:1.25rc3-alpine AS builder

ENV GOTOOLCHAIN=auto

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o bin/agent ./cmd/agent
RUN go build -o bin/worker ./cmd/worker
RUN go build -o bin/embedder ./cmd/embedder
RUN go build -o bin/api ./cmd/api

# Stage 2 — Run
FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ffmpeg ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /app/bin/ ./bin/

EXPOSE 8080
