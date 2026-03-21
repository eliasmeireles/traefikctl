BINARY_NAME=traefikctl
VERSION?=dev
BUILD_DIR=build
LDFLAGS=-ldflags "-X github.com/eliasmeireles/traefikctl/internal/cmd.Version=$(VERSION)"

.PHONY: build clean test lint fmt install

build:
	@echo "Building $(BINARY_NAME)..."
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/traefikctl

clean:
	@echo "Cleaning..."
	rm -rf $(BUILD_DIR)

test:
	@echo "Running tests..."
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

lint:
	@echo "Running linters..."
	golangci-lint run ./...

fmt:
	@echo "Formatting code..."
	gofmt -w .
	goimports -w .

install: build
	@echo "Installing $(BINARY_NAME)..."
	sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/$(BINARY_NAME)
	sudo chmod +x /usr/local/bin/$(BINARY_NAME)
	@echo "Installed to /usr/local/bin/$(BINARY_NAME)"
