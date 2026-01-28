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

run: build
	@./go-s3-uploader -bucket="s3.ungur.ro" -source=test/output -cachefile=test/.go3up.txt

cover:
	@go test -coverprofile=coverage.out
	@go tool cover -html=coverage.out

lint:
	@golangci-lint run ./...

clean:
	@rm -f go-s3-uploader go3up coverage.out

.PHONY: test build lint cover clean run
