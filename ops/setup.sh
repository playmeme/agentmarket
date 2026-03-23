#!/bin/bash
# setup.sh — One-time setup for AgentMarket on a fresh Ubuntu/Debian VPS

# IMPORTANT NOTE:
# This script isn't used.
# The app container is run using Podman Quadlet instead.
# See `~/.config/containers/systemd`.

set -e

REPO_URL="https://github.com/playmeme/agentmarket.git"
REPO_DIR="agentmarket.git"
IMAGE_NAME="webapp-image"
CONTAINER_NAME="webapp"
HOST_PORT=8080

echo "==> Installing Podman..."
apt-get update -qq
apt-get install -y podman

echo "==> Cloning repository..."
git clone "$REPO_URL" "$REPO_DIR"
cd "$REPO_DIR"

echo "==> Building container image..."
podman build -t "$IMAGE_NAME" -f Containerfile .

echo "==> Starting container on port $HOST_PORT..."
podman run -d \
  --name "$CONTAINER_NAME" \
  --restart=always \
  -p "$HOST_PORT":8080 \
  "$IMAGE_NAME"

echo ""
echo "Done. AgentMarket is running at http://$(hostname -I | awk '{print $1}'):$HOST_PORT"
echo "To update in the future, run: bash deploy.sh"
