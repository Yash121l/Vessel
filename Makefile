BINARY     := vessel
VERSION    := $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.1.0")
GOFLAGS    := -mod=mod
LDFLAGS    := -ldflags="-s -w -X github.com/Yash121l/Vessel/internal/cli.Version=$(VERSION)"
BUILD_DIR  := dist

.PHONY: build run clean test lint tidy release help

## build: compile for current platform
build:
	CGO_ENABLED=0 GOFLAGS="$(GOFLAGS)" go build $(LDFLAGS) -o $(BINARY) .

## run: build and run the server
run: build
	./$(BINARY) serve

## bootstrap: build and run system bootstrap (requires root)
bootstrap: build
	sudo ./$(BINARY) bootstrap

## test: run all tests
test:
	CGO_ENABLED=0 GOFLAGS="$(GOFLAGS)" go test ./... -v -race -timeout 60s

## vet: run go vet
vet:
	CGO_ENABLED=0 GOFLAGS="$(GOFLAGS)" go vet ./...

## lint: run golangci-lint
lint:
	golangci-lint run ./...

## tidy: tidy go modules
tidy:
	go mod tidy

## clean: remove build artifacts
clean:
	rm -rf $(BINARY) $(BUILD_DIR) tmp/

## release: cross-compile for Linux amd64, arm64, armv7
release:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOFLAGS="$(GOFLAGS)" GOOS=linux GOARCH=amd64       go build $(LDFLAGS) -o $(BUILD_DIR)/vessel_linux_amd64 .
	CGO_ENABLED=0 GOFLAGS="$(GOFLAGS)" GOOS=linux GOARCH=arm64       go build $(LDFLAGS) -o $(BUILD_DIR)/vessel_linux_arm64 .
	CGO_ENABLED=0 GOFLAGS="$(GOFLAGS)" GOOS=linux GOARCH=arm GOARM=7 go build $(LDFLAGS) -o $(BUILD_DIR)/vessel_linux_armv7 .
	@echo "Binaries written to $(BUILD_DIR)/"

## dev: run with hot reload (requires air: go install github.com/air-verse/air@latest)
dev:
	CGO_ENABLED=0 GOFLAGS="$(GOFLAGS)" air

## help: show available targets
help:
	@grep -E '^## ' Makefile | sed 's/## /  /'
