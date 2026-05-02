#!/bin/bash
set -e

echo "=== Diet Tracker Dev Startup ==="

# Kill existing
echo "Checking port 8080..."
sudo fuser -k 8080/tcp 2>/dev/null || true
sleep 1

# Rebuild if needed
if [ ! -f bin/diet-tracker-server ] || [ internal/** -nt bin/diet-tracker-server ]; then
    echo "Building Go server..."
    go build -o bin/diet-tracker-server ./cmd/server
fi

# Start Go server in background
echo "Starting Go server on :8080..."
DB_PATH=./data/diet.db ENV=development ./bin/diet-tracker-server &
SERVER_PID=$!

sleep 2

# Check if server is listening
if ! lsof -i :8080 >/dev/null 2>&1; then
    echo "ERROR: Server failed to start!"
    exit 1
fi

echo "✅ Go server started (PID: $SERVER_PID)"

# Start Vite
echo "Starting Vite dev server on :5173..."
cd ui/web
npm run dev
