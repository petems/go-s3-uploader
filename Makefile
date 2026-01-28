build:
	@go build

test:
	@go test -race -coverprofile=coverage.txt -covermode=atomic ./...

run: build
	@./go-s3-uploader -bucket="s3.ungur.ro" -source=test/output -cachefile=test/.go3up.txt

cover:
	@go test -coverprofile=coverage.out
	@go tool cover -html=coverage.out

clean:
	@rm -f go-s3-uploader go3up coverage.out

.PHONY: test build
