#!/bin/bash
# Simple script to build and run the operator locally
# Usage: ./run-go.sh [options]
#
# This script automatically builds the operator using build.sh before running.
# For manual builds, use: ./build.sh -o bin/manager main.go
#
# Options:
#   --log-level <level>    Set log level (error, info, debug, 0-10) [default: info]
#   --dev                  Enable development mode (console logs) [default: false]
#   --skip-build           Skip the build step (use existing binary)
#   --stop                 Stop the running operator and exit
#   --help                 Show this help message
#
# See BUILD.md for more information about build.sh and build options.

set -e

# Function to stop running operator
stop_operator() {
    local pid=$(pgrep -f "./bin/manager" | head -1)
    if [ -n "$pid" ]; then
        echo "Stopping operator (PID: $pid)..."
        kill "$pid" 2>/dev/null || true
        sleep 1
        # Force kill if still running
        if kill -0 "$pid" 2>/dev/null; then
            echo "Force stopping operator..."
            kill -9 "$pid" 2>/dev/null || true
        fi
        echo "✅ Operator stopped"
        return 0
    else
        echo "ℹ️  No operator process found"
        return 1
    fi
}

# Default values
LOG_LEVEL="${ZAP_LOG_LEVEL:-info}"
DEV_MODE="${ZAP_DEVEL:-false}"
SKIP_BUILD=false
STOP_ONLY=false

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --log-level)
            LOG_LEVEL="$2"
            shift 2
            ;;
        --dev)
            DEV_MODE="true"
            shift
            ;;
        --skip-build)
            SKIP_BUILD=true
            shift
            ;;
        --stop)
            STOP_ONLY=true
            shift
            ;;
        --help)
            echo "Usage: ./run-go.sh [options]"
            echo ""
            echo "This script automatically builds the operator using build.sh before running."
            echo "For manual builds, use: ./build.sh -o bin/manager main.go"
            echo ""
            echo "Options:"
            echo "  --log-level <level>    Set log level (error, info, debug, 0-10) [default: info]"
            echo "  --dev                  Enable development mode (console logs) [default: false]"
            echo "  --skip-build           Skip the build step (use existing binary)"
            echo "  --stop                 Stop the running operator and exit"
            echo "  --help                 Show this help message"
            echo ""
            echo "Environment variables:"
            echo "  ZAP_LOG_LEVEL          Override log level"
            echo "  ZAP_DEVEL              Override dev mode (true/false)"
            echo ""
            echo "See BUILD.md for more information about build.sh and build options."
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

# Handle --stop option
if [ "$STOP_ONLY" = true ]; then
    stop_operator
    exit 0
fi

# Stop any running operator before starting
if pgrep -f "./bin/manager" > /dev/null; then
    echo "⚠️  Operator is already running. Stopping it first..."
    stop_operator
    echo ""
fi

# Build the operator (unless skipped)
if [ "$SKIP_BUILD" = false ]; then
    echo "Building operator using build.sh..."
    echo "  (To skip build, use: ./run-go.sh --skip-build)"
    echo ""
    ./build.sh -o bin/manager main.go
    echo ""
else
    echo "Skipping build step (using existing binary)"
    if [ ! -f bin/manager ]; then
        echo "⚠️  Warning: bin/manager not found."
        echo "Building automatically using build.sh..."
        echo ""
        ./build.sh -o bin/manager main.go
        echo ""
    else
        echo ""
    fi
fi

# Run the operator
echo ""
echo "Starting operator with:"
echo "  LOG_LEVEL: $LOG_LEVEL"
echo "  DEV_MODE:  $DEV_MODE"
echo ""
echo "Press Ctrl+C to stop"
echo ""

ZAP_LOG_LEVEL="$LOG_LEVEL" ZAP_DEVEL="$DEV_MODE" ./bin/manager

