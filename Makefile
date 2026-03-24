.PHONY: build run clean test test-ui test-e2e test-e2e-ds3 help manifest manifest-test tools

# Embed manifest resource (required by lxn/walk)
manifest:
	go run github.com/akavel/rsrc@latest -manifest deathcounter.manifest -o rsrc.syso

# Embed manifest resource for tray UI tests (separate to avoid "too many .rsrc sections" on build)
manifest-test:
	go run github.com/akavel/rsrc@latest -manifest deathcounter.manifest -o internal/tray/rsrc_test.syso

# Install build tools
tools:
	go install github.com/akavel/rsrc@latest

# Build the application
build: manifest
	@echo "Building deathcounter..."
	go build -o deathcounter.exe -ldflags="-H windowsgui" .

# Build without hiding console (useful for debugging)
build-console: manifest
	@echo "Building deathcounter with console..."
	go build -o deathcounter.exe .

# Run the application (builds first)
run: build-console
	./deathcounter.exe

# Run tests
test:
	go test -count=1 ./internal/...

# Run UI tests — walk-based tray tests (requires Windows desktop session + manifest)
test-ui: manifest-test
	go test -tags e2e,ui -v -count=1 ./internal/tray/

# Run E2E tests — game-agnostic only (requires any supported game running on Windows)
test-e2e:
	go test -tags e2e -v -count=1 ./internal/memreader/

# Run E2E tests — DS3 (requires Dark Souls III running on Windows)
# Includes game-agnostic + DS3-specific memreader + monitor state machine tests
test-e2e-ds3:
	go test -tags e2e,ds3 -v -count=1 ./internal/memreader/ ./internal/monitor/

# Clean build artifacts
clean:
	rm -f deathcounter.exe deathcounter.db rsrc.syso internal/tray/rsrc_test.syso

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
	@echo "  make build          - Build the application (no console window)"
	@echo "  make build-console  - Build with console window (for debugging)"
	@echo "  make run            - Build and run the application"
	@echo "  make test           - Run tests"
	@echo "  make test-ui        - Run UI tests: walk tray (requires desktop session)"
	@echo "  make test-e2e       - Run E2E tests: game-agnostic (any game)"
	@echo "  make test-e2e-ds3   - Run E2E tests: DS3 (memreader + monitor)"
	@echo "  make clean          - Remove build artifacts"
	@echo "  make fmt            - Format code"
	@echo "  make vet            - Run go vet"
	@echo "  make lint           - Run linter"
	@echo "  make deps           - Download and tidy dependencies"
	@echo "  make manifest       - Embed Windows manifest resource"
	@echo "  make tools          - Install build tools (rsrc)"
	@echo ""
	@echo "E2E tag combinations (go test -tags ...):"
	@echo "  e2e       - Game-agnostic tests (attach, death count)"
	@echo "  e2e,ds3   - + DS3 event flags, AOB, stats, save identity, monitor phases"
	@echo "  e2e,ui    - Walk tray UI tests (requires desktop session + manifest)"
	@echo "  e2e,er    - + Elden Ring tests (future)"
	@echo "  e2e,ds2   - + Dark Souls II tests (future)"
