#!/usr/bin/env bash
set -euo pipefail

# Smoke E2E: start services (using docker compose), wait for API health,
# check prompts API (GET/POST), and verify prompts.json is written.

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
cd "$ROOT_DIR"

DATA_DIR=${DATA_DIR:-$PWD/data}
PROGRESS_DIR="$DATA_DIR/progress"
mkdir -p "$PROGRESS_DIR"

# Start compose (use existing docker-compose.yml)
docker compose up -d --build

# wait for api health
echo "Waiting for API health..."
for i in {1..30}; do
  if curl -fs http://localhost:8080/health >/dev/null 2>&1; then
    echo "API healthy"
    break
  fi
  sleep 2
  echo -n "."
  if [ "$i" -eq 30 ]; then
    echo
    echo "API did not become healthy in time"
    docker compose logs api --tail=200
    exit 1
  fi
done

# get current prompt
echo "GET /api/prompts"
curl -sS http://localhost:8080/api/prompts | jq || true

# post a new prompt
NEW_PROMPT='{"system_instruction":"smoke test instruction"}'
echo "POST /api/prompts"
curl -sS -X POST -H "Content-Type: application/json" -d "$NEW_PROMPT" http://localhost:8080/api/prompts

sleep 1

# verify prompts.json exists
if [ ! -f "$PROGRESS_DIR/prompts.json" ]; then
  echo "prompts.json not found in $PROGRESS_DIR"
  docker compose logs api --tail=200
  exit 1
fi

echo "Smoke test passed. Cleaning up..."
docker compose down

exit 0
