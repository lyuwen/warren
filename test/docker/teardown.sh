#!/bin/bash
# Teardown script for Warren Docker SSH test environment

set -e

echo "🧹 Tearing down Warren Docker SSH test environment..."

# Navigate to docker directory
cd "$(dirname "$0")"

# Use docker compose (v2) if available, otherwise docker-compose (v1)
if docker compose version &> /dev/null; then
    DOCKER_COMPOSE="docker compose"
else
    DOCKER_COMPOSE="docker-compose"
fi

# Stop and remove containers
echo "🛑 Stopping containers..."
$DOCKER_COMPOSE down

# Optional: Remove volumes
if [ "$1" == "--volumes" ] || [ "$1" == "-v" ]; then
    echo "🗑️  Removing volumes..."
    $DOCKER_COMPOSE down -v
fi

# Optional: Remove images
if [ "$1" == "--all" ] || [ "$1" == "-a" ]; then
    echo "🗑️  Removing images..."
    $DOCKER_COMPOSE down -v
    docker rmi warren-ssh-test 2>/dev/null || true
fi

echo "✅ Teardown complete!"
