SOVRABASE_DATA_DIR ?= ./data
SOVRABASE_LISTEN_ADDR ?= :6070
export SOVRABASE_DATA_DIR
export SOVRABASE_LISTEN_ADDR

.PHONY: build run test clean deps fmt lint dev swagger build-frontend install-hooks

BUILD_DIR=build
ifeq ($(OS),Windows_NT)
BINARY_NAME=sovrabase.exe
ifneq ($(findstring sh,$(SHELL)),)
USE_UNIX_SHELL = 1
else
ifneq ($(MSYSTEM),)
USE_UNIX_SHELL = 1
endif
endif
else
BINARY_NAME=sovrabase
USE_UNIX_SHELL = 1
endif

deps:
	go mod tidy
	go mod download

build-frontend:
	cd frontend && npm install --silent && npm run build
ifdef USE_UNIX_SHELL
	rm -rf internal/dashboard/dist
	cp -r frontend/dist internal/dashboard/dist
else
ifeq ($(OS),Windows_NT)
	@if exist internal\dashboard\dist rmdir /s /q internal\dashboard\dist
	xcopy /e /i /y frontend\dist internal\dashboard\dist >nul 2>&1
else
	rm -rf internal/dashboard/dist
	cp -r frontend/dist internal/dashboard/dist
endif
endif

# Point this repo at scripts/git-hooks/ (rebuilds dashboard on frontend commits).
install-hooks:
	git config core.hooksPath scripts/git-hooks
ifeq ($(OS),Windows_NT)
	@echo Git hooks installed. On Windows, ensure scripts/git-hooks/pre-commit is executable in Git Bash.
else
	chmod +x scripts/git-hooks/pre-commit
	@echo Git hooks installed.
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
