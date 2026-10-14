BIN := bin/findedmevents

.PHONY: build run run-http test lint tidy inspect

build:
	go build -o $(BIN) ./cmd/findedmevents

run:
	go run ./cmd/findedmevents

run-http:
	go run ./cmd/findedmevents -http :8080

test:
	go test ./...

lint:
	go vet ./...

tidy:
	go mod tidy

# Opens the MCP Inspector against the stdio server (requires Node).
inspect: build
	npx @modelcontextprotocol/inspector $(BIN)
