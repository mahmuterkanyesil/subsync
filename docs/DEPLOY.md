# Deployment Guide — CasaOS / Raspberry Pi

This document shows recommended steps to deploy Subsync to a Raspberry Pi running CasaOS (or directly with Docker Compose), how to set persistent volumes and env vars, and how to update images via CI.

Prerequisites
- A Raspberry Pi (64-bit recommended) with Docker and CasaOS installed, or at least Docker & Docker Compose.
- A container registry account (Docker Hub or GitHub Container Registry) and credentials.
- The project built as a multi-arch image and pushed to the registry (see `.github/workflows/docker-multiarch.yml`).

Quick summary
1. Build multi-arch image and push to your registry.
2. In CasaOS UI add an app using `yourname/subsync:tag` or run `docker compose` on the Pi.
3. Map volumes for `STATE_DB_PATH` and `PROGRESS_DIR` to host folders.
4. Provide necessary environment variables and ports.

Recommended image tags
- Use semantic version tags like `v1.2.0`. `latest` can be used for convenience but prefer explicit tags for production/deployed devices.

CasaOS (UI) installation steps
1. Build & push an image to your registry: `yourname/subsync:v1.0.0` (or `latest`).
2. Open CasaOS → App Store → "Install from Image" (or similar feature) and enter the image name.
3. Configure container settings:
   - Ports: Map `8080` → `8080` (host) if you want API/UI exposed.
   - Volumes: map a host folder to `/data` inside the container (recommended). Example host folders:
     - `/home/pi/subsync/data` → `/data`
   - Environment variables: see Env matrix below.
   - Restart policy: `unless-stopped`.
4. Start the container and check logs. Open `http://<pi-ip>:8080` to access the web UI.

Docker Compose (preferred for multi-service)
If you want to run all services (redis, api, worker, agent, embedder) on the Pi, use `docker-compose.yml` in this repo.

On the Pi:
```bash
# pull images (if using registry)
docker compose pull
# start services
docker compose up -d
```

Persistence & volumes
- Map `/data` to a host directory. The following items are stored under `/data` by default:
  - `state.db` (SQLite DB) — set by `STATE_DB_PATH`
  - `progress/` (progress files, prompts.json)
- Example compose volume mapping (in CasaOS or docker-compose):
  - Host `/home/pi/subsync/data` -> Container `/data`

Env matrix (important variables)
- `REDIS_URL` (default: `redis://redis:6379/0`) — if running external redis set accordingly.
- `STATE_DB_PATH` (default: `/data/state.db`) — path to SQLite DB inside container.
- `PROGRESS_DIR` (default: `/data/progress`) — where progress and prompts files are stored.
- `WATCH_DIRS` — colon-separated paths to watch (map host dirs accordingly).
- `API_PORT` (default: `8080`) — port api listens on.
- `LOG_FORWARD_URL` — internal API url for forwarded logs, set to `http://api:8080/api/internal/logs` in compose.
- `LOG_LEVEL` (default: `info`)
- `BATCH_SIZE`, `WORKER_CONCURRENCY` — tune for Pi's CPU/RAM.

Healthchecks
- `api` exposes `/api/health` endpoint. Ensure CasaOS health check (or compose) calls `http://localhost:8080/health`.

Updating and CI
- Use the included GitHub Actions workflow to build multi-arch images and push tags to your registry.
- Update flow:
  1. Create a release or push to `main` — CI builds image and pushes `yourname/subsync:vX.Y.Z`.
  2. In CasaOS, stop container, change image tag to new `vX.Y.Z`, pull and restart — or use an auto-updater (Watchtower) to pull `latest`.
  3. Verify application health (`/api/health`) and behaviour.

Rollback
- Keep previous image tags. To rollback, change the CasaOS image to the previous tag and restart.

Troubleshooting
- If ffmpeg missing or fails: check the runtime image used (Debian-based image includes `ffmpeg` via apt). Check container logs for `ffmpeg` errors.
- If SQLite errors appear after moving volumes: check file ownership and permissions inside container; ensure container user can write to `/data`.
- If tasks aren't being processed: confirm `redis` is reachable and `WORKER_CONCURRENCY` is adequate.

Security
- Do not expose the API to the internet without authentication. Use a reverse proxy (nginx) with basic auth or place the Pi behind a VPN.
- Keep registry credentials secret and use GitHub Actions secrets for CI.

Monitoring & housekeeping
- Periodically prune old progress files and monitor disk usage on the Pi.
- Consider limiting log retention if logs forwarded to the API in-memory buffer.

Appendix: Example minimal `docker run` for API only (not recommended for full setup)
```bash
docker run -d \
  -p 8080:8080 \
  -v /home/pi/subsync/data:/data \
  -e STATE_DB_PATH=/data/state.db \
  -e PROGRESS_DIR=/data/progress \
  yourname/subsync:latest
```

If you want, I can also add a tailored CasaOS step-by-step with screenshots, or commit a small `deploy.sh` helper script to automate the pull/restart process on the Pi.
