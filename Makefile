SOVRABASE_DATA_DIR ?= ./data
SOVRABASE_LISTEN_ADDR ?= :6070
export SOVRABASE_DATA_DIR
export SOVRABASE_LISTEN_ADDR

.PHONY: build run test clean deps fmt lint dev swagger

BINARY_NAME=sovrabase
BUILD_DIR=build

deps:
	go mod tidy
	go mod download

build: deps
	go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/sovrabase

run: build
	$(BUILD_DIR)/$(BINARY_NAME)

test:
	go test -v -race ./...

test-short:
	go test -short ./...

clean:
ifeq ($(OS),Windows_NT)
	@if exist $(BUILD_DIR) rmdir /s /q $(BUILD_DIR)
	@if exist data rmdir /s /q data
else
	rm -rf $(BUILD_DIR) data/
endif

fmt:
	go fmt ./...

lint:
	go vet ./...

dev: build
	$(BUILD_DIR)/$(BINARY_NAME)

swagger:
	swag init -g cmd/sovrabase/main.go -o docs/
	python3 scripts/fix-swagger-security.py docs/swagger.json
