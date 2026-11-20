#!/usr/bin/env bash
# restore.sh verifies checksums, pre-checks archive tables, and restores into a target database.
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: scripts/restore.sh --dump <path.pg_dump> [options]

Required:
  --dump <path>        Path to the custom-format .pg_dump archive file

Options:
  --url <dsn>          Target PostgreSQL connection URL
  --compose            Run restore inside the docker compose db container
  --compose-service    Name of compose db service (default: db)
  --compose-user       PostgreSQL user for compose exec (default: myknownworld)
  --compose-db         Target PostgreSQL database name for compose (default: myknownworld)
  --skip-checksum      Do not verify the accompanying .sha256 file
  -h, --help           Show this help message

Environment variables supported:
  DATABASE_URL         Target PostgreSQL connection URL
EOF
  exit 1
}

DUMP_PATH=""
DB_URL="${DATABASE_URL:-}"
USE_COMPOSE=false
COMPOSE_SERVICE="db"
COMPOSE_USER="myknownworld"
COMPOSE_DB="myknownworld"
CHECK_CHECKSUM=true

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dump)
      DUMP_PATH="$2"
      shift 2
      ;;
    --url)
      DB_URL="$2"
      shift 2
      ;;
    --compose)
      USE_COMPOSE=true
      shift
      ;;
    --compose-service)
      COMPOSE_SERVICE="$2"
      shift 2
      ;;
    --compose-user)
      COMPOSE_USER="$2"
      shift 2
      ;;
    --compose-db)
      COMPOSE_DB="$2"
      shift 2
      ;;
    --skip-checksum)
      CHECK_CHECKSUM=false
      shift
      ;;
    -h|--help)
      usage
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage
      ;;
  esac
done

if [ -z "$DUMP_PATH" ]; then
  echo "Error: --dump <path.pg_dump> is required." >&2
  usage
fi

if [ ! -f "$DUMP_PATH" ]; then
  echo "Error: Dump file not found: $DUMP_PATH" >&2
  exit 1
fi

# 1. Verify Checksum if present
CHECKSUM_FILE="${DUMP_PATH%.*}.sha256"
if [ "$CHECK_CHECKSUM" = true ] && [ -f "$CHECKSUM_FILE" ]; then
  echo "==> Verifying sha256 checksum..."
  DUMP_DIR="$(dirname "$DUMP_PATH")"
  DUMP_BASE="$(basename "$DUMP_PATH")"
  (cd "$DUMP_DIR" && sha256sum --check "$(basename "$CHECKSUM_FILE")")
elif [ "$CHECK_CHECKSUM" = true ]; then
  echo "Notice: Checksum file not found at ${CHECKSUM_FILE}; skipping checksum verification."
fi

# 2. Inspect archive TOC
echo "==> Inspecting archive table of contents..."
TOC_COUNT=0
if [ "$USE_COMPOSE" = true ]; then
  TOC_COUNT="$(docker compose exec -T "$COMPOSE_SERVICE" pg_restore --list < "$DUMP_PATH" | grep -v '^;' | grep -v '^$' | wc -l || true)"
else
  TOC_COUNT="$(pg_restore --list "$DUMP_PATH" | grep -v '^;' | grep -v '^$' | wc -l || true)"
fi
echo "  Found ${TOC_COUNT} catalog entries in archive"

# 3. Perform Restore
echo "==> Restoring into target database..."
START_TIME=$(date +%s%N 2>/dev/null || date +%s)

if [ "$USE_COMPOSE" = true ]; then
  docker compose exec -T "$COMPOSE_SERVICE" \
    pg_restore --clean --if-exists --no-owner --no-privileges \
      -U "$COMPOSE_USER" -d "$COMPOSE_DB" \
    < "$DUMP_PATH"
elif [ -n "$DB_URL" ]; then
  pg_restore --clean --if-exists --no-owner --no-privileges \
    --dbname="$DB_URL" "$DUMP_PATH"
elif [ -n "${PGDATABASE:-}" ]; then
  pg_restore --clean --if-exists --no-owner --no-privileges \
    "$DUMP_PATH"
else
  echo "Error: No target database specified. Provide --url, set DATABASE_URL, or use --compose." >&2
  exit 1
fi

END_TIME=$(date +%s%N 2>/dev/null || date +%s)
DURATION_MS=0
if [ ${#START_TIME} -gt 10 ] && [ ${#END_TIME} -gt 10 ]; then
  DURATION_MS=$(( (END_TIME - START_TIME) / 1000000 ))
fi

echo "==> Restore completed successfully in ${DURATION_MS} ms"
echo ""
echo "Important Operational Notes:"
echo "• If your application uses CMDB with CMDB_SECRET_PROVIDER=file, encrypted credential"
echo "  values in the database require the original CMDB_VAULT_KEY_BASE64 to be decrypted."
echo "• The restored database contains the schema and graph state as of the dump timestamp."
echo "• Startup will automatically verify and execute any newer migrations if restoring into"
echo "  a newer application version."
