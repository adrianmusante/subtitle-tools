SHELL=/bin/bash
MAKEFLAGS+=-s

BUILD_DIR:="$(PWD)/bin"
EXECUTABLE_NAME:=subtitle-tools
MAIN_PACKAGE:=./cmd/$(EXECUTABLE_NAME)

go-test:
	@echo "Running tests..."
	go test ./...

go-sum:
	rm -f go.sum && \
	go mod tidy

build:
	@echo "Building application..."
	@mkdir -p "$(BUILD_DIR)"
	go build -o "$(BUILD_DIR)/$(EXECUTABLE_NAME)" $(MAIN_PACKAGE)

build-windows:
	@echo "Building for Windows (amd64)..."
	@mkdir -p "$(BUILD_DIR)"
	GOOS=windows GOARCH=amd64 go build -o "$(BUILD_DIR)/$(EXECUTABLE_NAME).exe" $(MAIN_PACKAGE)

build-windows-arm64:
	@echo "Building for Windows (arm64)..."
	@mkdir -p "$(BUILD_DIR)"
	GOOS=windows GOARCH=arm64 go build -o "$(BUILD_DIR)/$(EXECUTABLE_NAME)-arm64.exe" $(MAIN_PACKAGE)

build-all: build build-windows build-windows-arm64
	@echo "Building for macOS (amd64)..."
	@mkdir -p "$(BUILD_DIR)"
	GOOS=darwin GOARCH=amd64 go build -o "$(BUILD_DIR)/$(EXECUTABLE_NAME)-macos-amd64" $(MAIN_PACKAGE)
	@echo "Building for macOS (arm64)..."
	GOOS=darwin GOARCH=arm64 go build -o "$(BUILD_DIR)/$(EXECUTABLE_NAME)-macos-arm64" $(MAIN_PACKAGE)
	@echo "All binaries built successfully!"

.PHONY: version
version:
	@go run ./cmd/subtitle-tools --version

.PHONY: go-test go-sum build build-windows build-windows-arm64 build-all release-tag
# Usage: make release-tag VERSION=1.2.3
.PHONY: release-tag
release-tag:
	@test -n "$(VERSION)" || (echo "VERSION is required (e.g. VERSION=1.2.3)" && exit 1)
	git tag -a "v$(VERSION)" -m "Release v$(VERSION)"
	git push origin "v$(VERSION)"
