#!/bin/bash

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(dirname "$SCRIPT_DIR")"  # parent of site-agent-built/
IMAGE_NAME="webapp-image"
SERVICE_NAME="webapp.service"

git fetch origin main

# Compare local HEAD with remote
LOCAL=$(git rev-parse HEAD)
REMOTE=$(git rev-parse origin/main)

if [ "$LOCAL" != "$REMOTE" ] || [ "$1" == "--force" ]; then
    echo "Changes detected or force flag set. Re-deploying..."
    
    git pull origin main
    podman build -t "$IMAGE_NAME" .    # Use --cgroup-manager if systemd bus issues persist
    systemctl --user restart "$SERVICE_NAME"
    
    echo "Deployment successful: $(date)"
else
    echo "No changes detected. Skipping build."
fi
