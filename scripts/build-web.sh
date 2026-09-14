#!/bin/bash
set -euo pipefail

cd "$(dirname "$0")/.."

echo "======================================"
echo "  go-stock 网页版构建"
echo "======================================"

if ! command -v go >/dev/null; then
  echo "错误：未找到 Go 编译器"
  exit 1
fi
if ! command -v npm >/dev/null; then
  echo "错误：未找到 Node.js/npm"
  exit 1
fi

echo "[1/3] 构建前端 (web)..."
cd frontend
npm install
npm run build:web
cd ..

echo "[2/3] 编译 go-stock-web..."
mkdir -p build/bin
CGO_ENABLED=0 go build -tags goweb -trimpath -ldflags "-s -w" -o build/bin/go-stock-web .

echo "[3/3] 完成"
echo "运行："
echo "  WEB_STATIC_DIR=frontend/dist ./build/bin/go-stock-web"
echo "然后打开 http://127.0.0.1:8080 ，使用 data/.web_auth_token 或 WEB_AUTH_TOKEN 登录"
echo
echo "Docker："
echo "  docker compose up -d --build"
