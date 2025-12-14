.PHONY: help test test-integration test-unit build clean docker-build lint fmt vet

# Default target
help:
	@echo "Available targets:"
	@echo "  test              - Run all tests"
	@echo "  test-unit         - Run unit tests only"
	@echo "  test-integration  - Run integration tests with testcontainer"
	@echo "  build             - Build the provider"
	@echo "  clean             - Clean build artifacts"
	@echo "  docker-build      - Build the test container image"
	@echo "  lint              - Run linters"
	@echo "  fmt               - Format code"
	@echo "  vet               - Run go vet"

# Run all tests
test:
	go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...

# Run unit tests only (fast)
test-unit:
	go test -v -short ./...

# Run integration tests with testcontainer
test-integration:
	go test -v -tags=integration ./internal/testcontainer/...

# Build the provider
build:
	go build -v ./...

# Clean build artifacts
clean:
	rm -rf ./bin
	rm -f coverage.txt
	go clean -cache -testcache

# Build the test container image manually
docker-build:
	docker build -t snitchdns-test:latest ./testcontainer

# Run linters
lint:
	golangci-lint run

# Format code
fmt:
	go fmt ./...

# Run go vet
vet:
	go vet ./...

# Install development tools
install-tools:
	@echo "Installing golangci-lint v2.7.2..."
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin v2.7.2
	@echo "Development tools installed successfully"
