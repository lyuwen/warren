.PHONY: all build clean test install help

# Default target
all: build

# Build all binaries
build:
	@echo "Building Warren binaries..."
	@go build -o warren ./cmd/warren
	@go build -o warren-tui ./cmd/warren-tui
	@go build -o warren-web ./cmd/warren-web
	@echo "✓ Build complete: warren, warren-tui, warren-web"

# Run tests
test:
	@echo "Running tests..."
	@go test ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	@go test -cover ./...

# Run tests verbosely
test-verbose:
	@echo "Running tests (verbose)..."
	@go test -v ./...

# Install binaries to $GOPATH/bin
install:
	@echo "Installing Warren binaries..."
	@go install ./cmd/warren
	@go install ./cmd/warren-tui
	@go install ./cmd/warren-web
	@echo "✓ Installed to $(shell go env GOPATH)/bin"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -f warren warren-tui warren-web
	@echo "✓ Clean complete"

# Run warren-web
run-web: build
	@echo "Starting Warren web interface..."
	@./warren-web

# Run warren-tui
run-tui: build
	@echo "Starting Warren TUI..."
	@./warren-tui

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...

# Run linter
lint:
	@echo "Running linter..."
	@golangci-lint run || echo "Note: golangci-lint not installed"

# Show help
help:
	@echo "Warren Build System"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build          Build all binaries (default)"
	@echo "  test           Run all tests"
	@echo "  test-coverage  Run tests with coverage"
	@echo "  test-verbose   Run tests with verbose output"
	@echo "  install        Install binaries to GOPATH/bin"
	@echo "  clean          Remove build artifacts"
	@echo "  run-web        Build and run warren-web"
	@echo "  run-tui        Build and run warren-tui"
	@echo "  fmt            Format code"
	@echo "  lint           Run linter"
	@echo "  help           Show this help message"
