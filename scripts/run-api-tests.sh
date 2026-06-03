#!/usr/bin/env bash
set -euo pipefail

DIR="$(cd "$(dirname "$0")/.." && pwd)"
HOST="${HOST:-http://localhost:8080}"
UID_VAL="${UID_VAL:-$(date +%s)$$}"
USE_SQLITE="${USE_SQLITE:-true}"

echo "Running Hurl API tests against $HOST (uid=$UID_VAL)"

# Start a local server if HOST is localhost and nothing is listening
if [[ "$HOST" == "http://localhost:8080" ]]; then
  if ! curl -sf "$HOST/api/ping/" > /dev/null 2>&1; then
    echo "Building server..."
    cd "$DIR"
    go build -o /tmp/realworld-server hello.go

    # Clear SQLite DB for a clean run
    if [[ "$USE_SQLITE" == "true" ]]; then
      rm -f ./data/gorm.db
    fi

    echo "Starting server..."
    /tmp/realworld-server &
    SERVER_PID=$!
    cleanup() {
      echo "Stopping server..."
      kill "$SERVER_PID" 2>/dev/null || true
      rm -f /tmp/realworld-server
    }
    trap cleanup EXIT

    echo "Waiting for server to be ready..."
    for i in {1..30}; do
      if curl -sf "$HOST/api/ping/" > /dev/null 2>&1; then
        echo "Server is up!"
        break
      fi
      sleep 1
    done
  fi
fi

# Select test files
HURL_DIR="$DIR/api/hurl"
FILES=("$@")
if [ ${#FILES[@]} -eq 0 ]; then
  FILES=("$HURL_DIR"/*.hurl)
fi

echo ""
echo "Running tests..."
hurl --test \
  --jobs 1 \
  --variable "host=$HOST" \
  --variable "uid=$UID_VAL" \
  "${FILES[@]}"
