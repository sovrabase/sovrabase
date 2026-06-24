.PHONY: build run test clean deps fmt lint dev

BINARY_NAME=sovrabase
BUILD_DIR=build
CGO_ENABLED=0

deps:
	go mod tidy
	go mod download

build: deps
	CGO_ENABLED=$(CGO_ENABLED) go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/sovrabase

run: build
	./$(BUILD_DIR)/$(BINARY_NAME)

test:
	CGO_ENABLED=$(CGO_ENABLED) go test -v -race ./...

test-short:
	CGO_ENABLED=$(CGO_ENABLED) go test -short ./...

clean:
	rm -rf $(BUILD_DIR) data/

fmt:
	go fmt ./...

lint:
	go vet ./...

dev: build
	SOVRABASE_DATA_DIR=./data SOVRABASE_LISTEN_ADDR=:6070 ./$(BUILD_DIR)/$(BINARY_NAME)
