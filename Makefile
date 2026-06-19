SHELL=/bin/bash
MAKEFLAGS+=-s

BUILD_DIR:="$(PWD)/bin"
EXECUTABLE_NAME:=subtitle-tools
MAIN_PACKAGE:=./cmd/$(EXECUTABLE_NAME)
FIX_CASES_DIR:=./internal/cli/testdata/fix/cases

go-test:
	@echo "Running tests..."
	go test ./...

# Runs fix CLI regression fixtures without updating expected outputs.
go-test-fix-regression:
	@echo "Running fix CLI regression fixtures..."
	go test ./internal/cli -run TestFixCLI_RegressionCases

# Lists available fix CLI regression cases.
go-test-fix-cases:
	@echo "Available fix CLI regression cases:"
	@find "$(FIX_CASES_DIR)" -mindepth 1 -maxdepth 1 -type d -exec basename {} \; | sort

# Runs a single fix CLI regression case.
# Usage: make go-test-fix-case CASE=strip-hi-safe-inplace
# If CASE is empty and fzf is installed, an interactive selector is shown.
go-test-fix-case:
	@CASE_NAME="$(CASE)"; \
	if [ -z "$$CASE_NAME" ]; then \
		if command -v fzf >/dev/null 2>&1; then \
			CASE_NAME="$$(find "$(FIX_CASES_DIR)" -mindepth 1 -maxdepth 1 -type d -exec basename {} \; | sort | fzf --prompt='Fix case> ' --height=40% --reverse --border --exit-0)"; \
		fi; \
	fi; \
	if [ -z "$$CASE_NAME" ]; then \
		echo "CASE is required (e.g. CASE=strip-hi-safe-inplace)"; \
		echo ""; \
		echo "Available cases:"; \
		find "$(FIX_CASES_DIR)" -mindepth 1 -maxdepth 1 -type d -exec basename {} \; | sort; \
		echo ""; \
		echo "Tip: install fzf or pass CASE explicitly."; \
		exit 1; \
	fi; \
	echo "Running fix CLI regression case: $$CASE_NAME"; \
	go test ./internal/cli -run "TestFixCLI_RegressionCases/$$CASE_NAME$$"

# Regenerates expected outputs for fix CLI regression fixtures.
go-test-fix-update-expected:
	@echo "Updating fix CLI regression expected fixtures..."
	UPDATE_FIX_GOLDEN=1 go test ./internal/cli -run TestFixCLI_RegressionCases

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

.PHONY: go-test go-test-fix-regression go-test-fix-cases go-test-fix-case go-test-fix-update-expected go-sum build build-windows build-windows-arm64 build-all release-tag
# Usage: make release-tag VERSION=1.2.3
.PHONY: release-tag
release-tag:
	@test -n "$(VERSION)" || (echo "VERSION is required (e.g. VERSION=1.2.3)" && exit 1)
	git tag -a "v$(VERSION)" -m "Release v$(VERSION)"
	git push origin "v$(VERSION)"
