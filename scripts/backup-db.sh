#!/usr/bin/env bash
# Back up the MyKnownWorld database before a risky change.
#
# Undo used to be a built-in feature; it was removed because it stored two full
# graph snapshots per write, which does not scale. The replacement is a real
# database backup taken before the change, plus logical deletion for business
# data (nodes are retired, never physically removed).
#
# Usage:
#   scripts/backup-db.sh                      # production default
#   BACKUP_DIR=~/backups/x scripts/backup-db.sh
#   PG_CONTAINER=hall-pgsql scripts/backup-db.sh
#
# The dump is verified by listing its table data before the script reports
# success, so a corrupt file is never mistaken for a usable backup.
set -euo pipefail

PG_CONTAINER="${PG_CONTAINER:-hall-pgsql}"
PG_USER="${PG_USER:-myknownworld}"
PG_DB="${PG_DB:-myknownworld}"
BACKUP_DIR="${BACKUP_DIR:-$HOME/backups/my-known-world}"
SSH_TARGET="${SSH_TARGET:-svc@192.168.208.130}"
LABEL="${1:-manual}"

mkdir -p "$BACKUP_DIR"
stamp="$(date +%Y%m%d-%H%M%S)"
out="$BACKUP_DIR/${stamp}-${LABEL}.dump"

echo "备份 $PG_DB（标签: $LABEL）→ $out"

# --format=custom 让 pg_restore 能选择性还原；--no-owner 让文件不依赖目标角色。
if ! ssh "$SSH_TARGET" "docker exec $PG_CONTAINER pg_dump -U $PG_USER -d $PG_DB --format=custom --no-owner" > "$out"; then
	echo "错误：pg_dump 失败，请检查容器 $PG_CONTAINER 是否在运行" >&2
	rm -f "$out"
	exit 1
fi

if [ ! -s "$out" ]; then
	echo "错误：备份文件为空" >&2
	rm -f "$out"
	exit 1
fi

# 校验：能列出内容才算可用。坏备份比没有备份更危险。
tables="$(ssh "$SSH_TARGET" "docker exec -i $PG_CONTAINER pg_restore --list" < "$out" 2>/dev/null | grep -c 'TABLE DATA' || true)"
if [ "${tables:-0}" -lt 10 ]; then
	echo "错误：备份只含 $tables 张表的数据，明显不完整" >&2
	rm -f "$out"
	exit 1
fi

echo "完成：$(du -h "$out" | cut -f1)，含 $tables 张表的数据"
echo "$out"
