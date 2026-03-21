#!/bin/bash

# NOTE:
# Currently this isn't used because Github Actions uses .github/workflows/deploy.yml instead

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(dirname "$SCRIPT_DIR")"
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
    systemctl --user daemon-reload
    systemctl --user restart "$SERVICE_NAME"
    podman image prune -f
    
    echo "Deployment successful: $(date)"
else
    echo "No changes detected. Skipping build."
fi
