SHELL=/bin/bash
MAKEFLAGS+=-s

BUILD_DIR:="$(PWD)/bin"
EXECUTABLE_NAME:=subtitle-tools

go-test:
	@echo "Running tests..."
	go test ./...

go-sum:
	rm -f go.sum && \
	go mod tidy

build:
	@echo "Building application..."
	@mkdir -p "$(BUILD_DIR)"
	go build -o "$(BUILD_DIR)/$(EXECUTABLE_NAME)" ./cmd/$(EXECUTABLE_NAME)

.PHONY: version
version:
	@go run ./cmd/subtitle-tools --version

# Creates an annotated tag "vX.Y.Z" and pushes it.
# Usage: make release-tag VERSION=1.2.3
.PHONY: release-tag
release-tag:
	@test -n "$(VERSION)" || (echo "VERSION is required (e.g. VERSION=1.2.3)" && exit 1)
	git tag -a "v$(VERSION)" -m "Release v$(VERSION)"
	git push origin "v$(VERSION)"
