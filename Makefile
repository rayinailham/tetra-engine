.PHONY: build run test lint migrate-up migrate-down tidy

# Build the binary
build:
	go build -o bin/tetra.exe ./cmd/tetra

# Run the application
run:
	go run ./cmd/tetra

# Run all tests
test:
	go test -v -race -count=1 ./...

# Run linter
lint:
	golangci-lint run ./...

# Download and tidy dependencies
tidy:
	go mod tidy

# Database migrations (requires golang-migrate CLI)
migrate-up:
	migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path migrations -database "$(DATABASE_URL)" down 1

# Run with .env
dev:
	@if exist .env (set /p dummy=<.env) & go run ./cmd/tetra
