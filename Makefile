# Version information
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u '+%Y-%m-%d_%H:%M:%S')

LDFLAGS := -X 'main.Version=$(VERSION)' \
           -X 'main.GitCommit=$(GIT_COMMIT)' \
           -X 'main.BuildDate=$(BUILD_DATE)'

all: build

build:
	@go build -ldflags "$(LDFLAGS)"

test:
	@AWS_SECRET_ACCESS_KEY=secret AWS_ACCESS_KEY_ID=secret go test -race -coverprofile=coverage.txt -covermode=atomic ./...

run: build
	@./go-s3-uploader -bucket="s3.ungur.ro" -source=test/output -cachefile=test/.go3up.txt

cover:
	@AWS_SECRET_ACCESS_KEY=secret AWS_ACCESS_KEY_ID=secret go test -coverprofile=coverage.out
	@go tool cover -html=coverage.out

clean:
	@rm -f go-s3-uploader go3up coverage.out

# LocalStack targets for acceptance testing
localstack-up:
	@docker-compose up -d
	@$(MAKE) localstack-health

localstack-down:
	@docker-compose down

localstack-health:
	@echo "Waiting for LocalStack to be ready..."
	@until curl -sf http://localhost:4566/_localstack/health | grep -q '"s3": "available"' 2>/dev/null; do \
		printf "."; \
		sleep 1; \
	done
	@echo "\nLocalStack is ready!"

test-acceptance: localstack-up
	@AWS_SECRET_ACCESS_KEY=test AWS_ACCESS_KEY_ID=test AWS_DEFAULT_REGION=us-east-1 \
		go test -v -tags=acceptance ./...

test-all: test test-acceptance

.PHONY: all test test-acceptance test-all build run cover clean localstack-up localstack-down localstack-health
