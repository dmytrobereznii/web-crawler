.DEFAULT_GOAL := build

fmt:
	go fmt ./...

vet: fmt
	go vet ./...

lint:
	golangci-lint run

build: vet
	go build

go: vet
	go run ./cmd/server

clean:
	go clean

test:
	go test -race ./..
