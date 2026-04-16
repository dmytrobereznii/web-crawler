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
	go test -v ./...

test-c:
	go test -v ./internal/crawler

race:
	go test -race ./...

d-rebuild:
	docker compose up -d --build

d-start:
	docker compose up -d

d-stop:
	docker compose down

d-logs:
	docker compose logs --tail=all -f
