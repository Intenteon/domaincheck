.PHONY: all help build server cli test test-verbose test-coverage lint check clean install run-server run-cli

BINARY_SERVER = domaincheck-server
BINARY_CLI = domaincheck

# Default target - show help
help:
	@echo "Domain Checker v2.0 - Make Targets"
	@echo ""
	@echo "Build targets:"
	@echo "  make build          - Build both server and CLI binaries"
	@echo "  make server         - Build server binary only"
	@echo "  make cli            - Build CLI binary only"
	@echo "  make install        - Install binaries to /usr/local/bin"
	@echo "  make clean          - Remove build artifacts"
	@echo ""
	@echo "Testing targets:"
	@echo "  make test           - Run all tests"
	@echo "  make test-verbose   - Run tests with verbose output"
	@echo "  make test-coverage  - Run tests with coverage report"
	@echo "  make lint           - Run linters (go vet + staticcheck if available)"
	@echo "  make check          - Run all checks (test + lint)"
	@echo ""
	@echo "Run targets:"
	@echo "  make run-server     - Build and run the server (port 8765)"
	@echo "  make run-cli        - Run CLI example (use: make run-cli ARGS='trucore')"
	@echo ""
	@echo "Meta targets:"
	@echo "  make all            - Build, test, and lint everything"

# Build everything
all: build test lint
	@echo "✓ Build, test, and lint complete"

# Build both binaries
build: server cli

# Build server
server:
	@echo "Building server..."
	go build -v -o $(BINARY_SERVER) ./cmd/server
	@echo "✓ Server built: $(BINARY_SERVER)"

# Build CLI
cli:
	@echo "Building CLI..."
	go build -v -o $(BINARY_CLI) ./cmd/cli
	@echo "✓ CLI built: $(BINARY_CLI)"

# Run all tests
test:
	@echo "Running all tests..."
	@go test ./...
	@echo "✓ All tests passed"

# Run tests with verbose output
test-verbose:
	@echo "Running tests (verbose)..."
	@go test -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	@echo ""
	@go test -cover ./...
	@echo ""
	@echo "Detailed coverage report:"
	@go test -coverprofile=coverage.out ./... > /dev/null 2>&1
	@go tool cover -func=coverage.out
	@echo ""
	@echo "Coverage file: coverage.out (use 'go tool cover -html=coverage.out' for HTML report)"

# Run linters
lint:
	@echo "Running go vet..."
	@go vet ./...
	@echo "✓ go vet passed"
	@if command -v staticcheck > /dev/null 2>&1; then \
		echo "Running staticcheck..."; \
		staticcheck ./...; \
		echo "✓ staticcheck passed"; \
	else \
		echo "ℹ  staticcheck not installed (optional: go install honnef.co/go/tools/cmd/staticcheck@latest)"; \
	fi

# Run all quality checks
check: test lint
	@echo "✓ All quality checks passed - ready for deployment"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -f $(BINARY_SERVER) $(BINARY_CLI)
	@rm -f coverage.out
	@echo "✓ Clean complete"

# Install binaries to /usr/local/bin
install: build
	@echo "Installing binaries to /usr/local/bin..."
	@cp $(BINARY_SERVER) /usr/local/bin/
	@cp $(BINARY_CLI) /usr/local/bin/
	@echo "✓ Installed: /usr/local/bin/$(BINARY_SERVER)"
	@echo "✓ Installed: /usr/local/bin/$(BINARY_CLI)"

# Run server
run-server: server
	@echo "Starting server on port 8765..."
	./$(BINARY_SERVER)

# Run CLI with arguments (usage: make run-cli ARGS='trucore priment')
run-cli: cli
	@if [ -z "$(ARGS)" ]; then \
		echo "Running CLI example..."; \
		./$(BINARY_CLI) intentixdf.com; \
	else \
		./$(BINARY_CLI) $(ARGS); \
	fi
