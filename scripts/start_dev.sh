#!/usr/bin/env bash
set -e

REPO_DIR="/home/dev/git-repo/github.com/QuietSugar/my-known-world"
cd "$REPO_DIR"

echo "=== 1. 检查 PostgreSQL 数据库容器 ==="
if ! docker ps --format '{{.Names}}' | grep -q 'hall-mkw-test-db'; then
    echo "正在启动 PostgreSQL 容器 hall-mkw-test-db..."
    docker start hall-mkw-test-db
    sleep 2
fi
echo "PostgreSQL 数据库已就绪。"

echo "=== 2. 编译并启动 Go 后端服务 (端口 8088) ==="
go build -o myknownworld-server ./cmd/myknownworld
pkill -f 'myknownworld-server' 2>/dev/null || true
nohup env ADDR=127.0.0.1:8088 ./myknownworld-server > /tmp/myknownworld.log 2>&1 &
echo "Go 后端服务已启动在 127.0.0.1:8088 (日志: /tmp/myknownworld.log)"

echo "=== 3. 启动前端 Vite 开发服务器 (端口 5173) ==="
cd "$REPO_DIR/web"
pkill -f 'vite.*5173' 2>/dev/null || true
nohup env API_PROXY_TARGET=http://127.0.0.1:8088 npm run dev -- --host 0.0.0.0 --port 5173 > /tmp/vite-dev.log 2>&1 &
echo "前端服务已启动在 http://0.0.0.0:5173 (日志: /tmp/vite-dev.log)"

echo "=== 启动完成 ==="
echo "访问地址: http://localhost:5173"
