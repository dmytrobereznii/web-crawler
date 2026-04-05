.DEFAULT_GOAL := build

fmt:
	go fmt ./...

vet: fmt
	go vet ./...

build: vet
	go build

go: vet
	go run ./cmd/server

clean:
	go clean

lint:
	golangci-lint run

test:
	go test -race ./...
