# tiny-claw Makefile
# 用法: make <target>

SHELL    := /bin/bash
MODULE   := github.com/aethelgards/tiny-claw
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS  := -s -w -X main.version=$(VERSION)

# ---------- 变量 ----------
BIN_DIR   := ./bin
CLAW_BIN  := $(BIN_DIR)/claw
LARK_BIN  := $(BIN_DIR)/larkbot
SRC_CLAW  := ./cmd/claw
SRC_LARK  := ./cmd/larkbot

# ---------- 默认目标 ----------
.PHONY: all
all: build  ## 构建所有二进制 (默认)

# ---------- 构建 ----------
.PHONY: build
build: claw larkbot  ## 构建 claw + larkbot

.PHONY: claw
claw:  ## 构建 CLI 入口
	@mkdir -p $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(CLAW_BIN) $(SRC_CLAW)
	@echo "✅ $(CLAW_BIN) 构建完成"

.PHONY: larkbot
larkbot:  ## 构建飞书机器人入口
	@mkdir -p $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(LARK_BIN) $(SRC_LARK)
	@echo "✅ $(LARK_BIN) 构建完成"

# ---------- 运行 ----------
.PHONY: run-claw
run-claw: claw  ## 运行 CLI (PROMPT 变量传参, 如: make run-claw PROMPT="hello")
ifndef PROMPT
	$(error PROMPT 不能为空, 用法: make run-claw PROMPT="你的提示词")
endif
	$(CLAW_BIN) $(PROMPT)

.PHONY: run-serve
run-serve: claw  ## 运行 Dashboard 服务 (PORT/DATADIR 可选)
	$(CLAW_BIN) serve --port $(or $(PORT),8080) --data-dir $(or $(DATADIR),.claw)

.PHONY: run-larkbot
run-larkbot: larkbot  ## 运行飞书机器人
	$(LARK_BIN)

# ---------- 安装 ----------
.PHONY: install
install: build  ## 安装到 GOPATH/bin
	cp $(CLAW_BIN) $(shell go env GOPATH)/bin/claw
	cp $(LARK_BIN) $(shell go env GOPATH)/bin/larkbot
	@echo "✅ 已安装到 $(shell go env GOPATH)/bin/"

# ---------- 质量检查 ----------
.PHONY: check
check: vet test  ## 运行全部检查

.PHONY: vet
vet:  ## go vet
	go vet ./...

.PHONY: test
test:  ## 运行测试
	go test -count=1 ./...

.PHONY: lint
lint:  ## golangci-lint (需安装)
	golangci-lint run ./...

# ---------- 清理 ----------
.PHONY: clean
clean:  ## 删除构建产物
	rm -rf $(BIN_DIR)
	@echo "🧹 清理完成"

.PHONY: help
help:  ## 显示帮助
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
