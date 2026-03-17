BINARY_NAME=tcpdns
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

.PHONY: all build clean test lint install release-snapshot help

all: build

## Build
build: ## Build for current platform
	@echo "Building $(BINARY_NAME) $(VERSION)..."
	go build -ldflags "$(LDFLAGS)" -trimpath -o bin/$(BINARY_NAME) ./cmd/tcpdns

build-all: ## Build for all platforms
	@echo "Building for all platforms..."
	GOOS=linux   GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -trimpath -o bin/$(BINARY_NAME)-linux-amd64 ./cmd/tcpdns
	GOOS=linux   GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -trimpath -o bin/$(BINARY_NAME)-linux-arm64 ./cmd/tcpdns
	GOOS=darwin  GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -trimpath -o bin/$(BINARY_NAME)-darwin-amd64 ./cmd/tcpdns
	GOOS=darwin  GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -trimpath -o bin/$(BINARY_NAME)-darwin-arm64 ./cmd/tcpdns
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -trimpath -o bin/$(BINARY_NAME)-windows-amd64.exe ./cmd/tcpdns
	@echo "Binaries written to bin/"

## Install
install: build ## Install to GOPATH/bin
	cp bin/$(BINARY_NAME) $(GOPATH)/bin/$(BINARY_NAME) 2>/dev/null || cp bin/$(BINARY_NAME) /usr/local/bin/$(BINARY_NAME)
	@echo "Installed $(BINARY_NAME) to PATH"

## Test
test: ## Run tests
	go test -v -race ./...

test-coverage: ## Run tests with coverage
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## Lint
lint: ## Run linter
	golangci-lint run ./...

## Clean
clean: ## Remove build artifacts
	rm -rf bin/ dist/ coverage.out coverage.html

## Release
release-snapshot: ## Build a snapshot release (no publish)
	goreleaser release --snapshot --clean

release-check: ## Check goreleaser config
	goreleaser check

## Development
run: build ## Build and run
	./bin/$(BINARY_NAME) $(ARGS)

fmt: ## Format code
	go fmt ./...

vet: ## Vet code
	go vet ./...

mod-tidy: ## Tidy modules
	go mod tidy

## Help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
