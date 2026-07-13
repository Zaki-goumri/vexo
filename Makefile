build:
	@go build -o bin/fs

build-client:
	@go build -o bin/client ./cmd/client

run:
	@./bin/fs

test:
	@go test ./... -v
