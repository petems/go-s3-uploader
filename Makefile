# Version information
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u '+%Y-%m-%d_%H:%M:%S')

LDFLAGS := -X 'main.Version=$(VERSION)' \
           -X 'main.GitCommit=$(GIT_COMMIT)' \
           -X 'main.BuildDate=$(BUILD_DATE)'

build:
	@go build -ldflags "$(LDFLAGS)"

test:
	@go test -race -coverprofile=coverage.txt -covermode=atomic ./...

# Run acceptance tests (requires LocalStack)
test-acceptance: localstack-up
	@echo "Waiting for LocalStack to be ready..."
	@$(MAKE) localstack-wait
	@go test -race -v -tags=acceptance ./... ; result=$$?; $(MAKE) localstack-down; exit $$result

# Wait for LocalStack to be healthy
localstack-wait:
	@echo "Waiting for LocalStack health..."
	@for i in 1 2 3 4 5 6 7 8 9 10; do \
		curl -sf http://localhost:4566/_localstack/health > /dev/null && break; \
		echo "Attempt $$i: LocalStack not ready, waiting..."; \
		sleep 2; \
	done

# Run all tests
test-all: test test-acceptance

# Start LocalStack
localstack-up:
	@docker-compose up -d localstack
	@echo "LocalStack starting on http://localhost:4566"

# Stop LocalStack
localstack-down:
	@docker-compose down

# Check LocalStack health
localstack-health:
	@curl -s http://localhost:4566/_localstack/health | jq .

run: build
	@./go-s3-uploader -bucket="s3.ungur.ro" -source=test/output -cachefile=test/.go3up.txt

cover:
	@go test -coverprofile=coverage.out
	@go tool cover -html=coverage.out

lint:
	@golangci-lint run ./...

clean:
	@rm -f go-s3-uploader go3up coverage.out coverage.txt
	@rm -rf localstack-data

.PHONY: test test-acceptance test-all build run cover lint clean localstack-up localstack-down localstack-wait localstack-health
