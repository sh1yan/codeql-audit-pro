# CodeQLAuditPro —— CodeQL 数据库、查询与审计一体化工作台
#
# 常用命令：
#   make dev            启动热重载开发（Wails + Vite）
#   make build          打包当前平台的桌面应用（Wails 2.16+）
#   make build-fast     跳过前端安装、只重建前端后打包
#   make test           跑全部 Go 测试
#   make typecheck      前端 TypeScript 类型检查
#
# 依赖：
#   Go 1.25+、Node 20+、pnpm 10+、Wails CLI 2.16+
#   Wails CLI 安装：go install github.com/wailsapp/wails/v2/cmd/wails@latest

APP      := CodeQLAuditPro
BIN_DIR  := build/bin
WAILS    ?= wails
PNPM     ?= pnpm
FRONTEND := frontend

.PHONY: help deps dev build build-fast run test test-core vet fmt tidy typecheck frontend \
        clean distclean package-mac package-mac-universal package-win package-linux fonts check

help:
	@echo "可用目标："
	@echo "  deps            安装前端依赖（pnpm install）"
	@echo "  dev             启动 Wails 热重载开发（自动打开窗口）"
	@echo "  build           打包当前平台桌面应用到 $(BIN_DIR)"
	@echo "  build-fast      跳过 pnpm install，重建前端后打包"
	@echo "  run             启动 Wails 开发模式（同 dev）"
	@echo "  test            跑全部 Go 测试"
	@echo "  test-core       只跑 internal/* 包测试"
	@echo "  typecheck       前端 TypeScript 类型检查"
	@echo "  vet / fmt / tidy"
	@echo "  frontend        仅构建前端到 $(FRONTEND)/dist"
	@echo "  fonts           打印 WebView 中文字体说明"
	@echo "  package-mac     macOS .app（Wails）"
	@echo "  package-win     Windows 可执行文件（建议在 Windows 上执行）"
	@echo "  package-linux   Linux 可执行文件（建议在 Linux 上执行）"
	@echo "  clean / distclean"

deps:
	cd $(FRONTEND) && $(PNPM) install

frontend:
	cd $(FRONTEND) && $(PNPM) build

typecheck:
	cd $(FRONTEND) && $(PNPM) exec tsc --noEmit

dev:
	$(WAILS) dev

build:
	$(WAILS) build

build-fast: frontend
	$(WAILS) build -s

run:
	$(WAILS) dev

test:
	go test ./...

test-core:
	go test ./internal/...

vet:
	go vet ./...

fmt:
	gofmt -l -w .

tidy:
	go mod tidy

fonts:
	go run . -fonts

package-mac:
	$(WAILS) build

package-mac-universal:
	$(WAILS) build -platform darwin/universal

package-win:
	$(WAILS) build -platform windows/amd64

package-linux:
	$(WAILS) build -platform linux/amd64

check: fmt vet typecheck test

clean:
	rm -rf $(BIN_DIR)

distclean: clean
	rm -rf $(FRONTEND)/dist $(FRONTEND)/node_modules
