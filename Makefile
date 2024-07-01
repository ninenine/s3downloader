BINARY_NAME=s3-downloader
BUILD_DIR=build
SOURCE=cmd/main.go

# Build for Linux (amd64 and arm64)
build-linux-amd64:
	GOARCH=amd64 GOOS=linux go build -buildmode=pie -ldflags="-s -w -extldflags '-static'" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(SOURCE)

build-linux-arm64:
	GOARCH=arm64 GOOS=linux go build -buildmode=pie -ldflags="-s -w -extldflags '-static'" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(SOURCE)

# Build for Windows (amd64 and arm64)
build-windows-amd64:
	GOARCH=amd64 GOOS=windows go build -buildmode=pie -ldflags="-s -w -extldflags '-static'" -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(SOURCE)

build-windows-arm64:
	GOARCH=arm64 GOOS=windows go build -buildmode=pie -ldflags="-s -w -extldflags '-static'" -o $(BUILD_DIR)/$(BINARY_NAME)-windows-arm64.exe $(SOURCE)

# Build for macOS (amd64 and arm64)
build-macos-amd64:
	GOARCH=amd64 GOOS=darwin go build -buildmode=pie -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-macos-amd64 $(SOURCE)

build-macos-arm64:
	GOARCH=arm64 GOOS=darwin go build -buildmode=pie -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-macos-arm64 $(SOURCE)

# Build for FreeBSD (amd64 and arm64)
build-freebsd-amd64:
	GOARCH=amd64 GOOS=freebsd go build -buildmode=pie -ldflags="-s -w -extldflags '-static'" -o $(BUILD_DIR)/$(BINARY_NAME)-freebsd-amd64 $(SOURCE)

build-freebsd-arm64:
	GOARCH=arm64 GOOS=freebsd go build -buildmode=pie -ldflags="-s -w -extldflags '-static'" -o $(BUILD_DIR)/$(BINARY_NAME)-freebsd-arm64 $(SOURCE)

# Build for all architectures and OSes
build-all: build-linux-amd64 build-linux-arm64 build-windows-amd64 build-windows-arm64 build-macos-amd64 build-macos-arm64 build-freebsd-amd64 build-freebsd-arm64
