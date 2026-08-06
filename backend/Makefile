.PHONY: build run test lint migrate clean docker-up docker-down

BINARY_NAME=casino-server
BUILD_DIR=./bin

build:
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/server

run: build
	$(BUILD_DIR)/$(BINARY_NAME)

test:
	go test -v -race -cover ./...

lint:
	golangci-lint run ./...

migrate:
	go run ./cmd/migrate

clean:
	rm -rf $(BUILD_DIR)

docker-up:
	docker-compose up -d --build

docker-down:
	docker-compose down

tidy:
	go mod tidy

vet:
	go vet ./...
