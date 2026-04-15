# Subsync — Deployment Quickstart

This guide shows how to run Subsync as a dockerized service suitable for NAS/VM/Raspberry Pi.

Prerequisites
- Docker & Docker Compose (v2+) installed
- Host directories containing media (TV/Movies)

Quickstart
1. Copy environment file and edit paths:

```bash
cp .env.sample .env
# edit .env to set DATA_DIR, TV_DIR, MOVIES_DIR
```

2. Start services (build first run):

```bash
docker compose up -d --build
```

3. Check status:

```bash
docker compose ps
docker compose logs api --tail=100
```

4. Open Web UI: http://<host>:8080/
- Logs: `/logs`
- Settings: `/settings` to add/enable watch directories

Recommended host mounts (example `.env` values):
- `DATA_DIR` -> persistent state.db and progress files
- `TV_DIR`, `MOVIES_DIR` -> mount media library read-only

Low-power profile
- Use `docker-compose.override.yml` to set conservative defaults for CPU-limited devices.

Healthchecks
- Each service includes a simple process-based `healthcheck` in `docker-compose.yml`.

Troubleshooting
- If logs say `No such container` when referencing old IDs, the container was recreated; use `docker compose ps` to see current IDs.
- If translations appear stuck: check Redis connectivity and worker logs.
- If ffmpeg fails: ensure `ffmpeg` binary exists in container (Dockerfile installs it).

Advanced
- For multi-instance production, migrate to Postgres instead of SQLite (not covered here).
- Add a reverse proxy (Traefik/nginx) for TLS and auth if exposing API to the internet.
