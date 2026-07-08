#!/usr/bin/env bash
# 构建所有平台的 npm 包
set -euo pipefail
cd "$(dirname "$0")/.."

VERSION="${1:-1.0.0}"
LDFLAGS="-s -w"
GOEXE="$(go env GOEXE)"

echo "=== 构建 ctfcode v${VERSION} npm 包 ==="

# 构建当前平台二进制
echo "构建 linux/amd64..."
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "$LDFLAGS" -o "npm/platforms/linux-x64/bin/ctfcode" ./cmd/ctfcode

echo "构建 linux/arm64..."
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "$LDFLAGS" -o "npm/platforms/linux-arm64/bin/ctfcode" ./cmd/ctfcode

echo "构建 darwin/amd64..."
GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "$LDFLAGS" -o "npm/platforms/darwin-x64/bin/ctfcode" ./cmd/ctfcode

echo "构建 darwin/arm64..."
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "$LDFLAGS" -o "npm/platforms/darwin-arm64/bin/ctfcode" ./cmd/ctfcode

echo "构建 windows/amd64..."
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "$LDFLAGS" -o "npm/platforms/win32-x64/bin/ctfcode.exe" ./cmd/ctfcode

echo "构建 windows/arm64..."
GOOS=windows GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "$LDFLAGS" -o "npm/platforms/win32-arm64/bin/ctfcode.exe" ./cmd/ctfcode

echo ""
echo "=== 构建完成 ==="
echo ""
echo "发布到 npm:"
echo "  # 1. 登录 npm"
echo "  npm login"
echo ""
echo "  # 2. 发布平台包（先发布平台包，再发布主包）"
echo '  for pkg in npm/platforms/*/; do'
echo '    (cd "$pkg" && npm publish --access public)'
echo '  done'
echo ""
echo "  # 3. 更新主包的 optionalDependencies 版本号"
echo "  # 4. 发布主包"
echo "  cd npm/deepseek-ctfcode && npm publish --access public"
echo ""
echo "安装测试:"
echo "  npm install -g deepseek-ctfcode"
echo "  ctfcode --version"
