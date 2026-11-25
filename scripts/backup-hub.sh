#!/usr/bin/env bash
# hub 变动期高频备份：在每次批量改动前执行。
#
# 用法：
#   ./backup-hub.sh              # 备份 hub 仓库中所有已修改/新增的文件
#   ./backup-hub.sh <tag>        # 同上，并在目录名里带上标签
#
# 产物：~/backups/hub/<日期>-<时分秒>[-<tag>]/<原相对路径>
# 只备份改动过的文件（git 视角），不备份 node_modules 等未跟踪产物。
set -euo pipefail

# Override to back up another checkout: HUB_ROOT=/path/to/hub ./backup-hub.sh
HUB_ROOT="${HUB_ROOT:-/home/dev/git-repo/codeup.aliyun.com/5ed719afd1d1abe63b55e7da/hub}"
BACKUP_ROOT="$HOME/backups/hub"
tag="${1:-}"
stamp="$(date +%Y%m%d-%H%M%S)"
dest="$BACKUP_ROOT/$stamp${tag:+-$tag}"

cd "$HUB_ROOT"

# 收集所有需要备份的路径：已跟踪的改动 + 未跟踪的新文件。
mapfile -t files < <(
  git status --porcelain=v1 --untracked-files=all \
    | sed 's/^...//' \
    | sed 's/.* -> //' \
    | grep -v '^$'
)

if [ ${#files[@]} -eq 0 ]; then
  echo "没有需要备份的改动"
  exit 0
fi

mkdir -p "$dest"
count=0
for f in "${files[@]}"; do
  if [ -f "$f" ]; then
    mkdir -p "$dest/$(dirname "$f")"
    cp -p "$f" "$dest/$f"
    count=$((count + 1))
  fi
done

echo "已备份 $count 个文件到 $dest"
