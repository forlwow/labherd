# ===== 变量 =====
MODULE  := github.com/forlwow/labherd
# 版本号：有 tag 用 tag，没有就用提交哈希；有未提交改动会带 -dirty
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X $(MODULE)/internal/common/version.Version=$(VERSION)
BIN_DIR := bin
CMDS    := labherd-agent labherd-controller

.DEFAULT_GOAL := help
.PHONY: all proto proto-lint fmt lint test tidy-check build clean hooks help $(CMDS)

# ===== 组合目标 =====
all: fmt lint test build ## 提交前完整检查

# ===== 代码生成与接口检查 =====
proto: ## 由 .proto 生成 gen/
	buf generate

proto-lint: ## proto 风格检查 + 与 main 比较兼容性
	buf lint
	buf breaking --against '.git#branch=main'

# ===== 格式、检查、测试 =====
fmt: ## 格式化代码
	golangci-lint fmt

lint: ## 静态检查（含依赖方向）
	golangci-lint run ./...

test: ## 单元测试（竞争检测 + 覆盖率）
	go test -race -coverprofile=coverage.out ./...

tidy-check: ## 检查 go.mod/go.sum 是否已整理
	go mod tidy
	git diff --exit-code go.mod go.sum

# ===== 构建 =====
build: $(CMDS) ## 构建全部程序

$(CMDS):
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$@ ./cmd/$@

# ===== 杂项 =====
clean: ## 删除构建产物
	rm -rf $(BIN_DIR) coverage.out

hooks: ## 启用仓库内的 git hooks
	git config core.hooksPath .githooks

help: ## 列出所有目标
	@grep -E '^[a-zA-Z_-]+:.*## ' $(MAKEFILE_LIST) | \
	  awk 'BEGIN{FS=":.*## "}{printf "  %-12s %s\n", $$1, $$2}'
