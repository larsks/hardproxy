.PHONY: build test clean fmt

BINARY_NAME=hardproxy

build:
	go build -o $(BINARY_NAME) ./cmd/hardproxy

test:
	go test ./... -v

fmt:
	go fmt ./...

clean:
	rm -f $(BINARY_NAME)
	go clean

all: fmt build test
