.PHONY: build run clean test install

# 项目名称
BINARY_NAME=video-exporter

# 构建
build:
	go build -o $(BINARY_NAME) *.go

# 运行
run:
	go run *.go

# 清理
clean:
	go clean
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_NAME)-*

# 测试
test:
	go test -v ./...

# 安装依赖
install:
	go mod download

# 跨平台编译
build-all: build-linux build-windows build-mac

build-linux:
	GOOS=linux GOARCH=amd64 go build -o $(BINARY_NAME)-linux main.go

build-windows:
	GOOS=windows GOARCH=amd64 go build -o $(BINARY_NAME).exe main.go

build-mac:
	GOOS=darwin GOARCH=amd64 go build -o $(BINARY_NAME)-mac main.go

# 格式化代码
fmt:
	go fmt ./...

# 代码检查
lint:
	golangci-lint run

# 帮助
help:
	@echo "可用命令:"
	@echo "  make build       - 编译项目"
	@echo "  make run         - 运行项目"
	@echo "  make clean       - 清理编译文件"
	@echo "  make test        - 运行测试"
	@echo "  make install     - 安装依赖"
	@echo "  make build-all   - 跨平台编译"
	@echo "  make fmt         - 格式化代码"
	@echo "  make lint        - 代码检查"
