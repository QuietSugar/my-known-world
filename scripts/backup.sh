#!/usr/bin/env bash
# backup.sh creates a verified pg_dump -Fc archive, sha256 checksum, and a sanitized metadata manifest.
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: scripts/backup.sh [options]

Options:
  --out-dir <dir>      Output directory for backup artifacts (default: ./backups)
  --url <dsn>          PostgreSQL connection URL (e.g. postgres://user:pass@host:5432/dbname)
  --compose            Run pg_dump inside the docker compose db container
  --compose-service    Name of compose db service (default: db)
  --compose-user       PostgreSQL user for compose exec (default: myknownworld)
  --compose-db         PostgreSQL db name for compose exec (default: myknownworld)
  -h, --help           Show this help message

Environment variables supported:
  DATABASE_URL         PostgreSQL connection URL
  BACKUP_OUT_DIR       Default output directory
EOF
  exit 1
}

OUT_DIR="${BACKUP_OUT_DIR:-./backups}"
DB_URL="${DATABASE_URL:-}"
USE_COMPOSE=false
COMPOSE_SERVICE="db"
COMPOSE_USER="myknownworld"
COMPOSE_DB="myknownworld"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --out-dir)
      OUT_DIR="$2"
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
    -h|--help)
      usage
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage
      ;;
  esac
done

mkdir -p "$OUT_DIR"
TIMESTAMP="$(date -u +%Y%m%d_%H%M%SZ)"
BASE_NAME="myknownworld_backup_${TIMESTAMP}"
DUMP_FILE="${OUT_DIR}/${BASE_NAME}.pg_dump"
CHECKSUM_FILE="${OUT_DIR}/${BASE_NAME}.sha256"
MANIFEST_FILE="${OUT_DIR}/${BASE_NAME}_manifest.json"

START_TIME=$(date +%s%N 2>/dev/null || date +%s)

echo "==> Creating PostgreSQL custom-format dump..."
if [ "$USE_COMPOSE" = true ]; then
  # Stream dump from inside the Compose database container
  docker compose exec -T "$COMPOSE_SERVICE" \
    pg_dump -U "$COMPOSE_USER" -d "$COMPOSE_DB" \
      --format=custom --no-owner --no-privileges \
    > "$DUMP_FILE"
  SOURCE_DB="$COMPOSE_DB"
elif [ -n "$DB_URL" ]; then
  pg_dump --dbname="$DB_URL" --format=custom --no-owner --no-privileges > "$DUMP_FILE"
  SOURCE_DB="$(echo "$DB_URL" | sed -E 's|.*\/(.+)(\?.*)?|\1|')"
elif [ -n "${PGDATABASE:-}" ]; then
  pg_dump --format=custom --no-owner --no-privileges > "$DUMP_FILE"
  SOURCE_DB="${PGDATABASE}"
else
  echo "Error: No database connection specified. Provide --url, set DATABASE_URL, or use --compose." >&2
  exit 1
fi

END_TIME=$(date +%s%N 2>/dev/null || date +%s)
DURATION_MS=0
if [ ${#START_TIME} -gt 10 ] && [ ${#END_TIME} -gt 10 ]; then
  DURATION_MS=$(( (END_TIME - START_TIME) / 1000000 ))
fi

# Verify dump is readable with pg_restore list
echo "==> Verifying archive integrity..."
if [ "$USE_COMPOSE" = true ]; then
  docker compose exec -T "$COMPOSE_SERVICE" pg_restore --list < "$DUMP_FILE" > /dev/null
else
  pg_restore --list "$DUMP_FILE" > /dev/null
fi

# Generate checksum
echo "==> Generating sha256 checksum..."
(cd "$OUT_DIR" && sha256sum "$(basename "$DUMP_FILE")" > "$(basename "$CHECKSUM_FILE")")

# Extract sanitized configuration metadata (NO SECRET VALUES!)
HAS_VAULT_KEY=false
if [ -n "${CMDB_VAULT_KEY_BASE64:-}" ]; then
  HAS_VAULT_KEY=true
fi

HAS_WRITE_TOKEN=false
if [ -n "${MYKNOWNWORLD_WRITE_TOKEN:-}" ]; then
  HAS_WRITE_TOKEN=true
fi

CMDB_PROVIDER="${CMDB_SECRET_PROVIDER:-none}"

APP_VERSION="unknown"
if [ -f "VERSION" ]; then
  APP_VERSION="$(cat VERSION | tr -d '[:space:]')"
fi

DUMP_SIZE="$(wc -c < "$DUMP_FILE" | tr -d '[:space:]')"

cat <<EOF > "$MANIFEST_FILE"
{
  "timestamp": "${TIMESTAMP}",
  "version": "${APP_VERSION}",
  "sourceDatabase": "${SOURCE_DB}",
  "dumpFile": "$(basename "$DUMP_FILE")",
  "checksumFile": "$(basename "$CHECKSUM_FILE")",
  "format": "custom",
  "sizeBytes": ${DUMP_SIZE},
  "durationMs": ${DURATION_MS},
  "configurationMetadata": {
    "cmdbSecretProvider": "${CMDB_PROVIDER}",
    "hasCmdbVaultKey": ${HAS_VAULT_KEY},
    "hasWriteToken": ${HAS_WRITE_TOKEN}
  },
  "restoreNotes": [
    "Restore with scripts/restore.sh --dump <file> --url <target_dsn>.",
    "If CMDB secret provider is 'file', the restored credentials require the same CMDB_VAULT_KEY_BASE64."
  ]
}
EOF

echo "==> Backup complete in ${DURATION_MS} ms"
echo "  Archive:  ${DUMP_FILE} (${DUMP_SIZE} bytes)"
echo "  Checksum: ${CHECKSUM_FILE}"
echo "  Manifest: ${MANIFEST_FILE}"
