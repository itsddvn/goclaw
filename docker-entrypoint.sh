#!/bin/sh
set -e

case "${1:-serve}" in
  serve)
    # Managed mode: auto-run migrations before starting
    if [ "$GOCLAW_MODE" = "managed" ] && [ -n "$GOCLAW_POSTGRES_DSN" ]; then
      echo "Managed mode: running migrations..."
      /app/goclaw migrate up --migrations-dir "$GOCLAW_MIGRATIONS_DIR" || \
        echo "Migration warning (may already be up-to-date)"
    fi
    exec /app/goclaw
    ;;
  migrate)
    shift
    exec /app/goclaw migrate "$@"
    ;;
  onboard)
    exec /app/goclaw onboard
    ;;
  version)
    exec /app/goclaw version
    ;;
  *)
    exec /app/goclaw "$@"
    ;;
esac
