.PHONY: run build test lint docker-up docker-down load-test migrate clean

# Build binary
build:
	go build -ldflags="-s -w" -o bin/auth-service ./cmd/server

# Run locally (requires local Postgres + Redis)
run: build
	./bin/auth-service

# Run all tests
test:
	go test -v -race -cover ./...

# Lint with golangci-lint
lint:
	golangci-lint run ./...

# Start full stack with Docker Compose
docker-up:
	docker compose up -d --build

# Stop Docker Compose
docker-down:
	docker compose down -v

# Run database migration (requires running Postgres)
migrate:
	psql "$(DB_URL)" -f migrations/001_create_users.sql

# Run K6 load test (requires running app)
load-test:
	k6 run k6/load-test.js

# Clean build artifacts
clean:
	rm -rf bin/
