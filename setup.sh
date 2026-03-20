#!/bin/bash
# setup.sh — One-time setup for AgentMarket on a fresh Ubuntu/Debian VPS
# Usage: bash setup.sh
# Replace REPO_URL below with the actual git repository URL before running.

set -e

REPO_URL="REPO_URL"          # e.g. https://github.com/youruser/agentmarket.git
REPO_DIR="agentmarket"
IMAGE_NAME="agentmarket"
CONTAINER_NAME="agentmarket"
HOST_PORT=80

echo "==> Installing Podman..."
apt-get update -qq
apt-get install -y podman

echo "==> Cloning repository..."
git clone "$REPO_URL" "$REPO_DIR"
cd "$REPO_DIR/site-agent-built"

echo "==> Building container image..."
podman build -t "$IMAGE_NAME" -f Containerfile .

echo "==> Starting container on port $HOST_PORT..."
podman run -d \
  --name "$CONTAINER_NAME" \
  --restart=always \
  -p "$HOST_PORT":80 \
  "$IMAGE_NAME"

echo ""
echo "Done. AgentMarket is running at http://$(hostname -I | awk '{print $1}'):$HOST_PORT"
echo "To update in the future, run: bash deploy.sh"
