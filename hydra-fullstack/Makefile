.PHONY: build run dev test docker deploy logs status secrets db-shell clean proto lint fmt

APP_NAME=hydra-fullstack
PORT=8080

# Build the server binary
build:
	@echo "Building $(APP_NAME)..."
	@mkdir -p bin
	@go build -o bin/server cmd/server/main.go
	@echo "✓ Build complete: bin/server"

# Run the server locally
run:
	@go run cmd/server/main.go

# Run in development mode with hot reload
# Requires: go install github.com/air-verse/air@latest
dev:
	@which air > /dev/null || (echo "Installing air..."; go install github.com/air-verse/air@latest)
	@air

# Run with Discord webhook (set env var first)
# Usage: DISCORD_WEBHOOK=https://... make run-with-alerts
run-with-alerts:
	@DISCORD_WEBHOOK=$(DISCORD_WEBHOOK) REDIS_ADDR=localhost:6379 go run cmd/server/main.go

# Run all tests
test:
	@go test ./... -v -race -coverprofile=coverage.out
	@go tool cover -html=coverage.out -o coverage.html
	@echo "✓ Coverage report: coverage.html"

# Generate protobuf code
proto:
	@which protoc > /dev/null || (echo "Error: protoc not installed"; exit 1)
	@protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/render.proto
	@echo "✓ Protobuf generated"

# Build Docker image
docker:
	@docker build -t $(APP_NAME):latest .
	@echo "✓ Docker image built: $(APP_NAME):latest"

# Run Docker container locally
docker-run:
	@docker run -p 8080:8080 -p 9090:9090 -e DISCORD_WEBHOOK=$(DISCORD_WEBHOOK) $(APP_NAME):latest

# Deploy to Fly.io
deploy:
	@flyctl deploy --app $(APP_NAME)

# View Fly.io logs
logs:
	@flyctl logs --app $(APP_NAME)

# View Fly.io status
status:
	@flyctl status --app $(APP_NAME)

# List Fly.io secrets
secrets:
	@flyctl secrets list --app $(APP_NAME)

# Open SQLite shell
db-shell:
	@sqlite3 hydra.db

# Format Go code
fmt:
	@gofmt -w -s .
	@echo "✓ Code formatted"

# Run linter
lint:
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..."; go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	@golangci-lint run ./...

# Clean build artifacts
clean:
	@rm -rf bin/ uploads/ coverage.out coverage.html
	@go clean
	@echo "✓ Cleaned"

# Full CI pipeline
ci: fmt lint test build
	@echo "✓ CI pipeline complete"
