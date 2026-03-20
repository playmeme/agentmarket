#!/bin/bash
# deploy.sh — Pull latest code and redeploy AgentMarket
# Run from anywhere; the script locates the repo relative to itself.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(dirname "$SCRIPT_DIR")"  # parent of site-agent-built/
IMAGE_NAME="agentmarket"
CONTAINER_NAME="agentmarket"
HOST_PORT=80

echo "==> Pulling latest code..."
cd "$REPO_DIR"
git pull

echo "==> Building new container image..."
cd "$SCRIPT_DIR"
podman build -t "$IMAGE_NAME" -f Containerfile .

echo "==> Stopping old container (if running)..."
podman stop "$CONTAINER_NAME" 2>/dev/null || true
podman rm "$CONTAINER_NAME" 2>/dev/null || true

echo "==> Starting new container..."
podman run -d \
  --name "$CONTAINER_NAME" \
  --restart=always \
  -p "$HOST_PORT":80 \
  "$IMAGE_NAME"

echo ""
echo "Deployed. AgentMarket is running at http://$(hostname -I | awk '{print $1}'):$HOST_PORT"
