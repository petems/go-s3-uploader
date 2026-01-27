build:
	@go build

test:
	@AWS_SECRET_ACCESS_KEY=secret AWS_ACCESS_KEY_ID=secret go test -race -coverprofile=coverage.txt -covermode=atomic ./...

run: build
	@./go-s3-uploader -bucket="s3.ungur.ro" -source=test/output -cachefile=test/.go3up.txt

cover:
	@AWS_SECRET_ACCESS_KEY=secret AWS_ACCESS_KEY_ID=secret go test -coverprofile=coverage.out
	@go tool cover -html=coverage.out

clean:
	@rm -f go-s3-uploader go3up coverage.out

.PHONY: test build
