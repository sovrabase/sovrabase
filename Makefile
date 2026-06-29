SOVRABASE_DATA_DIR ?= ./data
SOVRABASE_LISTEN_ADDR ?= :6070
export SOVRABASE_DATA_DIR
export SOVRABASE_LISTEN_ADDR

.PHONY: build run test clean deps fmt lint dev swagger build-frontend

BUILD_DIR=build
ifeq ($(OS),Windows_NT)
BINARY_NAME=sovrabase.exe
else
BINARY_NAME=sovrabase
endif

deps:
	go mod tidy
	go mod download

build-frontend:
ifeq ($(OS),Windows_NT)
	cd frontend && npm install --silent && npm run build
	@if exist internal\dashboard\dist rmdir /s /q internal\dashboard\dist
	xcopy /e /i /y frontend\dist internal\dashboard\dist >nul 2>&1
else
	cd frontend && npm install --silent && npm run build && rm -rf ../internal/dashboard/dist && cp -r dist ../internal/dashboard/dist
endif

build: build-frontend deps
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
