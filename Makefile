.PHONY: build run clean test help

# Build the application
build:
	@echo "Building deathcounter..."
	go build -o deathcounter.exe -ldflags="-H windowsgui" .

# Build without hiding console (useful for debugging)
build-console:
	@echo "Building deathcounter with console..."
	go build -o deathcounter.exe .

# Run the application (builds first)
run: build-console
	./deathcounter.exe

# Run tests
test:
	go test ./...

# Clean build artifacts
clean:
	rm -f deathcounter.exe deathcounter.db

# Format code
fmt:
	go fmt ./...

# Run go vet
vet:
	go vet ./...

# Run linter (requires golangci-lint)
lint:
	golangci-lint run

# Download dependencies
deps:
	go mod download
	go mod tidy

# Help
help:
	@echo "Available commands:"
	@echo "  make build         - Build the application (no console window)"
	@echo "  make build-console - Build with console window (for debugging)"
	@echo "  make run           - Build and run the application"
	@echo "  make test          - Run tests"
	@echo "  make clean         - Remove build artifacts"
	@echo "  make fmt           - Format code"
	@echo "  make vet           - Run go vet"
	@echo "  make lint          - Run linter"
	@echo "  make deps          - Download and tidy dependencies"
