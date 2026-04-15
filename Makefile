.PHONY: build test vet clean

BINARY_NAME := gitbatch
BUILD_DIR := .

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "dev")
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X main.Version=$(VERSION) \
           -X main.Commit=$(COMMIT) \
           -X main.Date=$(DATE)

build:
	go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/gitbatch

test:
	go test ./...

vet:
	go vet ./...

clean:
	rm -f $(BUILD_DIR)/$(BINARY_NAME)
	rm -rf dist/
