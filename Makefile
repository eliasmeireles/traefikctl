.PHONY: build clean install test lint fmt help version version-push

BINARY_NAME=traefikctl
BUILD_DIR=build
GO=go
GOFLAGS=-v
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_DATE=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS=-ldflags "-s -w -X github.com/eliasmeireles/traefikctl/internal/cmd.Version=$(VERSION) -X github.com/eliasmeireles/traefikctl/internal/cmd.BuildDate=$(BUILD_DATE)"

help:
	@echo "Available targets:"
	@echo "  build          - Build the binary"
	@echo "  clean          - Remove build artifacts"
	@echo "  install        - Install the binary to /usr/local/bin"
	@echo "  test           - Run tests with coverage"
	@echo "  lint           - Run linters"
	@echo "  fmt            - Format code"
	@echo "  version        - Show current version"
	@echo "  version-push   - Push version tag to trigger release"

build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/traefikctl

clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@$(GO) clean

install: build
	@echo "Installing $(BINARY_NAME) to /usr/local/bin..."
	@sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/
	@sudo chmod +x /usr/local/bin/$(BINARY_NAME)
	@echo "Installed successfully"

test:
	@echo "Running tests..."
	$(GO) test -v -race -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

lint:
	@echo "Running linters..."
	@which golangci-lint > /dev/null || (echo "golangci-lint not found, install it from https://golangci-lint.run/usage/install/" && exit 1)
	golangci-lint run ./...

fmt:
	@echo "Formatting code..."
	$(GO) fmt ./...
	@which goimports > /dev/null && goimports -w . || echo "goimports not found, skipping"

version:
	@echo $(VERSION)

version-push:
	@if [ -z "$(TAG)" ]; then \
		echo "Error: TAG is required"; \
		echo "Usage: make version-push TAG=v0.1.0"; \
		exit 1; \
	fi
	@echo "Pushing tag $(TAG)..."
	@git push origin $(TAG)
	@echo "Tag $(TAG) pushed. GitHub Actions will build the release."
