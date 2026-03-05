#!/usr/bin/env bash
set -euo pipefail

# Worktree-aware dev environment launcher.
# Derives deterministic, non-colliding ports from the current directory name
# so multiple worktrees can run in parallel with isolated databases.
#
# Modes:
#   ./dev.sh              Start PostgreSQL + Go backend (frontend devs: just run Vite)
#   ./dev.sh --go         Start PostgreSQL only (fullstack devs: run Go + Vite manually)
#   ./dev.sh --down       Stop and remove containers

DIR_NAME=$(basename "$PWD")
HASH=$(printf '%s' "$DIR_NAME" | cksum | awk '{print $1}')
OFFSET=$((HASH % 100))

export DB_PORT=$((5432 + OFFSET))
export BACKEND_PORT=$((8090 + OFFSET))
VITE_PORT=$((5173 + OFFSET))
export VITE_API_URL="http://localhost:${BACKEND_PORT}"
export COMPOSE_PROJECT_NAME="togglerino-${DIR_NAME}"
export DATABASE_URL="postgres://togglerino:togglerino@localhost:${DB_PORT}/togglerino?sslmode=disable"

echo "=== Togglerino Dev Environment ==="
echo "Directory:    $DIR_NAME"
echo "Project:      $COMPOSE_PROJECT_NAME"
echo "PostgreSQL:   localhost:${DB_PORT}"
echo "Go backend:   localhost:${BACKEND_PORT}"
echo "Vite:         localhost:${VITE_PORT}"
echo "DATABASE_URL: $DATABASE_URL"
echo "=================================="

if [[ "${1:-}" == "--down" ]]; then
    echo "Stopping dev environment..."
    docker compose -f docker-compose.dev.yml down
    exit 0
fi

if [[ "${1:-}" == "--go" ]]; then
    # Fullstack mode: start only PostgreSQL, dev runs Go directly
    echo "Starting PostgreSQL..."
    docker compose -f docker-compose.dev.yml up -d postgres

    echo "Waiting for PostgreSQL..."
    until docker compose -f docker-compose.dev.yml exec -T postgres pg_isready -U togglerino > /dev/null 2>&1; do
        sleep 0.5
    done
    echo "PostgreSQL is ready."
    echo ""
    echo "Start the backend and frontend manually:"
    echo ""
    echo "  # Terminal 1: Go backend"
    echo "  DATABASE_URL=\"$DATABASE_URL\" PORT=$BACKEND_PORT LOG_FORMAT=text CORS_ORIGINS=\"http://localhost:${VITE_PORT}\" go run ./cmd/togglerino"
    echo ""
    echo "  # Terminal 2: Vite dev server"
    echo "  cd web && VITE_API_URL=\"$VITE_API_URL\" npm run dev -- --port $VITE_PORT"
    echo ""
    echo "To stop: ./dev.sh --down"
else
    # Default: start PostgreSQL + Go backend in Docker (frontend-only workflow)
    echo "Starting PostgreSQL + Go backend..."
    docker compose -f docker-compose.dev.yml up -d --build

    echo "Waiting for PostgreSQL..."
    until docker compose -f docker-compose.dev.yml exec -T postgres pg_isready -U togglerino > /dev/null 2>&1; do
        sleep 0.5
    done
    echo "Backend is running at http://localhost:${BACKEND_PORT}"
    echo ""
    echo "Start the Vite dev server:"
    echo ""
    echo "  cd web && VITE_API_URL=\"$VITE_API_URL\" npm run dev -- --port $VITE_PORT"
    echo ""
    echo "To stop: ./dev.sh --down"
fi
