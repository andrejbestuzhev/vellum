#!/usr/bin/env bash
# Rolls out the checked-out commit on the server: scripts/vault-deploy.sh <ref>
# Runs from the clone in /opt/apps/vault-mcp after `git checkout <ref>`
# (see .github/workflows/vault-deploy.yml). Builds the image on the server,
# restarts the container and waits for its health check.
set -euo pipefail

REF="${1:?usage: scripts/vault-deploy.sh <ref>}"
cd "$(dirname "$0")/.."

export COMPOSE_FILE=docker-compose.vault.yml
VAULT_MCP_VERSION="$(git rev-parse --short HEAD)"
export VAULT_MCP_VERSION
SERVICE=vault-mcp
STATE_FILE=.git/vault-deployed-ref  # inside .git: never shows as a change

PREV_REF=""
if [[ -f "$STATE_FILE" ]]; then
    PREV_REF="$(cat "$STATE_FILE")"
fi

on_error() {
    echo
    echo "!!! Deploy of $REF failed."
    if [[ -n "$PREV_REF" && "$PREV_REF" != "$REF" ]]; then
        echo "Roll back:"
        echo "  cd $(pwd) && git checkout --force $PREV_REF && ./scripts/vault-deploy.sh $PREV_REF"
    fi
}
trap on_error ERR

echo "==> Deploy $REF ($VAULT_MCP_VERSION, previous: ${PREV_REF:-none})"

[[ -f .env ]] || { echo ".env not found in $(pwd)" >&2; false; }
[[ -d vault ]] || { echo "vault/ not found in $(pwd) (owner 65532:65532)" >&2; false; }

echo "==> Build"
docker compose build

echo "==> Up"
docker compose up -d --remove-orphans

echo "==> Health check"
for _ in $(seq 1 30); do
    if docker compose exec -T "$SERVICE" /vellum -healthcheck >/dev/null 2>&1; then
        echo "$REF" > "$STATE_FILE"
        docker image prune -f >/dev/null
        echo "==> OK: $REF"
        exit 0
    fi
    sleep 2
done

echo "Health check failed for 60 s"
docker compose logs --tail 50 "$SERVICE"
false
