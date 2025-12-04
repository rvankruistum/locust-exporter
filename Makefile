# Makefile for locust_exporter

.PHONY: all build test lint clean docker docker-run help

# Binary name
BINARY_NAME := locust_exporter

# Build flags
BUILD_FLAGS := -a -tags netgo
LDFLAGS := -s -w

# Go commands
GO := go
GOFMT := gofmt
DOCKER := docker

all: test build

## help: Show this help message
help:
	@echo 'Usage:'
	@echo '  make <target>'
	@echo ''
	@echo 'Targets:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'

## build: Build the binary
build:
	@echo ">> building binary"
	$(GO) build $(BUILD_FLAGS) -ldflags="$(LDFLAGS)" -o $(BINARY_NAME) .
	@echo ">> binary built: $(BINARY_NAME)"

## test: Run tests
test:
	@echo ">> running tests"
	$(GO) test -v -race -cover ./...

## test-coverage: Run tests with coverage report
test-coverage:
	@echo ">> running tests with coverage"
	$(GO) test -v -race -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo ">> coverage report: coverage.html"

## lint: Run linters
lint:
	@echo ">> running linters"
	@which golangci-lint > /dev/null || (echo "golangci-lint not installed. Run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest" && exit 1)
	golangci-lint run ./...

## fmt: Format code
fmt:
	@echo ">> formatting code"
	$(GOFMT) -w -s .

## vet: Run go vet
vet:
	@echo ">> running go vet"
	$(GO) vet ./...

## clean: Clean build artifacts
clean:
	@echo ">> cleaning"
	rm -f $(BINARY_NAME)
	rm -f coverage.out coverage.html
	$(GO) clean

## docker: Build Docker image
docker:
	@echo ">> building docker image"
	$(DOCKER) build -t $(BINARY_NAME):latest .

## docker-run: Run Docker container locally
docker-run: docker
	@echo ">> running docker container"
	$(DOCKER) run --rm -p 9646:9646 $(BINARY_NAME):latest --locust.uri=http://localhost:8089

## run: Run the exporter locally
run: build
	@echo ">> running $(BINARY_NAME)"
	./$(BINARY_NAME) --locust.uri=http://localhost:8089

## deps: Download dependencies
deps:
	@echo ">> downloading dependencies"
	$(GO) mod download
	$(GO) mod verify

## tidy: Tidy dependencies
tidy:
	@echo ">> tidying dependencies"
	$(GO) mod tidy

## check: Run all checks (fmt, vet, lint, test)
check: fmt vet lint test
	@echo ">> all checks passed"
