#!/bin/bash
# Setup script for Warren Docker SSH test environment

set -e

echo "🐳 Setting up Warren Docker SSH test environment..."

# Check if Docker is installed
if ! command -v docker &> /dev/null; then
    echo "❌ Docker is not installed. Please install Docker first."
    exit 1
fi

# Check if docker compose is installed (v2 or v1)
if ! docker compose version &> /dev/null && ! command -v docker-compose &> /dev/null; then
    echo "❌ docker compose is not installed. Please install Docker Compose first."
    exit 1
fi

# Use docker compose (v2) if available, otherwise docker-compose (v1)
if docker compose version &> /dev/null; then
    DOCKER_COMPOSE="docker compose"
else
    DOCKER_COMPOSE="docker-compose"
fi

# Navigate to docker directory
cd "$(dirname "$0")"

# Build and start container
echo "📦 Building Docker image..."
$DOCKER_COMPOSE build

echo "🚀 Starting container..."
$DOCKER_COMPOSE up -d

# Wait for SSH to be ready
echo "⏳ Waiting for SSH service to be ready..."
max_attempts=30
attempt=0
while [ $attempt -lt $max_attempts ]; do
    if docker exec warren-ssh-test pgrep sshd > /dev/null 2>&1; then
        echo "✅ SSH service is ready!"
        break
    fi
    attempt=$((attempt + 1))
    sleep 1
done

if [ $attempt -eq $max_attempts ]; then
    echo "❌ SSH service failed to start within 30 seconds"
    docker-compose logs
    exit 1
fi

# Test SSH connection
echo "🔐 Testing SSH connection..."
if sshpass -p testpass ssh -o StrictHostKeyChecking=no -p 2222 testuser@localhost "echo 'SSH connection successful'" 2>/dev/null; then
    echo "✅ SSH connection test passed!"
else
    echo "⚠️  SSH connection test failed (sshpass may not be installed)"
    echo "   Try manually: ssh -p 2222 testuser@localhost"
    echo "   Password: testpass"
fi

# Create test tmux sessions
echo "📺 Creating test tmux sessions..."
docker exec -u testuser warren-ssh-test tmux new-session -d -s test-session
docker exec -u testuser warren-ssh-test tmux new-window -t test-session -n window1
docker exec -u testuser warren-ssh-test tmux send-keys -t test-session:0.0 'echo "Test pane 0"' C-m
docker exec -u testuser warren-ssh-test tmux send-keys -t test-session:1.0 'echo "Test pane 1"' C-m

echo "✅ Test tmux sessions created!"

# Display status
echo ""
echo "📊 Container Status:"
$DOCKER_COMPOSE ps

echo ""
echo "📺 Tmux Sessions:"
docker exec -u testuser warren-ssh-test tmux list-sessions

echo ""
echo "✅ Setup complete!"
echo ""
echo "Next steps:"
echo "  1. Test SSH: ssh -p 2222 testuser@localhost (password: testpass)"
echo "  2. Run integration tests: go test ./test/integration/..."
echo "  3. Stop container: cd test/docker && $DOCKER_COMPOSE down"
