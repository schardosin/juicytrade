#!/bin/bash
set -e

ASTONISH_DIR="/root/juicytrade/.astonish"

echo "🛑 Stopping all services..."

# Kill supervisor processes by reading their PID files
for pid_file in "$ASTONISH_DIR"/*.supervisor.pid; do
    if [ -f "$pid_file" ]; then
        pid=$(cat "$pid_file")
        service_name=$(basename "$pid_file" .supervisor.pid)
        if ps -p "$pid" > /dev/null 2>&1; then
            echo "  Killing $service_name (supervisor PID: $pid)..."
            kill -- -"$pid" 2>/dev/null || true
        fi
        rm -f "$pid_file"
    fi
done

# Fallback: kill any remaining background processes
pkill -f "./server" || true
pkill -f "vite" || true
pkill -f "node" || true

sleep 1

echo "✅ All services stopped"
exit 0
