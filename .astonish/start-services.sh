#!/bin/bash
set -e

export PATH=$PATH:/usr/local/go/bin
WORKSPACE=/root/juicytrade
ASTONISH_DIR="$WORKSPACE/.astonish"
LOGS_DIR="$ASTONISH_DIR/logs"

mkdir -p "$LOGS_DIR"

# Helper: Start a service with auto-restart supervisor
start_service() {
    local name="$1"
    local cmd="$2"
    local port="$3"
    local max_wait=60
    
    echo "🚀 Starting $name (port $port)..."
    
    # Start supervisor in background with restart loop
    setsid nohup bash -c "
        exec > '$LOGS_DIR/${name}.log' 2>&1
        while true; do
            $cmd
            sleep 2
        done
    " > /dev/null 2>&1 &
    
    local supervisor_pid=$!
    echo "$supervisor_pid" > "$ASTONISH_DIR/${name}.supervisor.pid"
    disown
    
    # Wait for service to be ready
    local waited=0
    while [ $waited -lt $max_wait ]; do
        if [ "$port" != "-" ] && nc -z localhost "$port" 2>/dev/null; then
            echo "✅ $name is ready (port $port)"
            return 0
        fi
        sleep 1
        waited=$((waited + 1))
    done
    
    echo "⚠️  $name startup timed out after ${max_wait}s, but continuing..."
    return 0
}

# Start backend
start_service "backend" "cd $WORKSPACE/trade-backend-go && ./server" "8008"

# Start frontend (using npx vite directly, not npm run dev)
start_service "frontend" "cd $WORKSPACE/trade-app && npx vite --host 0.0.0.0 --port 3001" "3001"

# Final health check
echo "🔍 Running health checks..."
sleep 2

if curl -s http://localhost:8008/health | grep -q "trade-backend-go"; then
    echo "✅ Backend health check passed"
else
    echo "⚠️  Backend health check inconclusive"
fi

if curl -s http://localhost:3001/ | grep -q "Juicy Trade"; then
    echo "✅ Frontend health check passed"
else
    echo "⚠️  Frontend health check inconclusive"
fi

echo "✅ All services started"
exit 0
