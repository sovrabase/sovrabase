.PHONY: build run test clean docs

build:
	go build -o bin/server ./cmd/server

run:
	go run ./cmd/server

test:
	go test ./... -v

clean:
	rm -rf bin/*

docs:
	swag init --parseDependency --parseInternal -g cmd/server/main.go -d ./ 2>&1
