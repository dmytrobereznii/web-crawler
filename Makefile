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

create-migration:
	docker run --rm -v $(CURDIR)/migrations:/migrations migrate/migrate create -ext sql -dir /migrations -seq $(c)

up:
	docker compose up -d

down:
	docker compose down -v

up-build: down
	docker compose up -d --build

d-logs:
	docker compose logs --tail=all -f
