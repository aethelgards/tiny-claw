#!/usr/bin/env bash
# tiny-claw 构建脚本
# 用法: ./scripts/build.sh [command]
#   无参数  → 构建所有二进制
#   claw    → 仅构建 CLI
#   larkbot → 仅构建飞书机器人
#   run     → 运行 CLI (第二个参数为 prompt)
#   serve   → 启动 Dashboard
#   clean   → 清理构建产物
#   install → 安装到 GOPATH/bin
#   check   → 运行 vet + test

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

BIN_DIR="$ROOT_DIR/bin"
MODULE="github.com/aethelgards/tiny-claw"

# 版本信息
VERSION=$(git -C "$ROOT_DIR" describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS="-s -w -X main.version=$VERSION"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

info()  { echo -e "${GREEN}✅ $*${NC}"; }
warn()  { echo -e "${YELLOW}⚠️  $*${NC}"; }
error() { echo -e "${RED}❌ $*${NC}" >&2; }
die()   { error "$@"; exit 1; }

# ---------- 构建函数 ----------
build_claw() {
    mkdir -p "$BIN_DIR"
    echo "🔨 构建 claw..."
    go build -ldflags "$LDFLAGS" -o "$BIN_DIR/claw" "$ROOT_DIR/cmd/claw"
    info "$BIN_DIR/claw 构建完成"
}

build_larkbot() {
    mkdir -p "$BIN_DIR"
    echo "🔨 构建 larkbot..."
    go build -ldflags "$LDFLAGS" -o "$BIN_DIR/larkbot" "$ROOT_DIR/cmd/larkbot"
    info "$BIN_DIR/larkbot 构建完成"
}

build_all() {
    build_claw
    build_larkbot
}

# ---------- 清理 ----------
do_clean() {
    rm -rf "$BIN_DIR"
    info "清理完成"
}

# ---------- 安装 ----------
do_install() {
    build_all
    local gopath
    gopath=$(go env GOPATH)
    cp "$BIN_DIR/claw"   "$gopath/bin/claw"
    cp "$BIN_DIR/larkbot" "$gopath/bin/larkbot"
    info "已安装到 $gopath/bin/"
}

# ---------- 质量检查 ----------
do_check() {
    echo "🔍 go vet..."
    go vet ./...
    info "go vet 通过"
    echo "🧪 运行测试..."
    go test -count=1 ./...
    info "测试通过"
}

# ---------- 运行 ----------
run_claw() {
    build_claw
    local prompt="${1:?用法: $0 run \"你的提示词\"}"
    "$BIN_DIR/claw" "$prompt"
}

run_serve() {
    build_claw
    local port="${PORT:-8080}"
    local datadir="${DATADIR:-.claw}"
    "$BIN_DIR/claw" serve --port "$port" --data-dir "$datadir"
}

run_larkbot() {
    build_larkbot
    "$BIN_DIR/larkbot"
}

# ---------- 主逻辑 ----------
usage() {
    cat <<EOF
tiny-claw 构建脚本

用法:
  $0              构建所有二进制
  $0 claw         仅构建 CLI
  $0 larkbot      仅构建飞书机器人
  $0 run <prompt> 运行 CLI
  $0 serve        启动 Dashboard (PORT/DATADIR 环境变量可选)
  $0 larkbot-run  运行飞书机器人
  $0 install      安装到 GOPATH/bin
  $0 check        运行 vet + test
  $0 clean        清理构建产物
  $0 help         显示帮助
EOF
}

cd "$ROOT_DIR"

case "${1:-}" in
    claw)       build_claw ;;
    larkbot)    build_larkbot ;;
    run)        run_claw "${2:-}" ;;
    serve)      run_serve ;;
    larkbot-run) run_larkbot ;;
    install)    do_install ;;
    check)      do_check ;;
    clean)      do_clean ;;
    help|-h|--help) usage ;;
    "")         build_all ;;
    *)          die "未知命令: $1  (使用 '$0 help' 查看用法)" ;;
esac
