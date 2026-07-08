BINARY  := hhb
CMD     := ./cmd/hhb
GALLERY := gallery
GALLERY_CMD := ./cmd/gallery
BIN_DIR := bin
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: all build gallery test lint vet fmt fmt-check run run-gallery clean tidy release-snapshot

all: build

## build: compile the hhb binary into ./bin
build:
	@mkdir -p $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY) $(CMD)

## gallery: compile the component gallery demo into ./bin
gallery:
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(GALLERY) $(GALLERY_CMD)

## test: run the full test suite with the race detector
test:
	go test -race ./...

## vet: run go vet
vet:
	go vet ./...

## fmt: format all Go sources in place
fmt:
	gofmt -w .

## fmt-check: fail if any Go source is not gofmt-clean
fmt-check:
	@test -z "$$(gofmt -l .)" || { echo "gofmt needed on:"; gofmt -l .; exit 1; }

## lint: run golangci-lint (must be installed)
lint:
	golangci-lint run

## tidy: sync go.mod/go.sum
tidy:
	go mod tidy

## run: build then run hhb
run: build
	./$(BIN_DIR)/$(BINARY)

## run-gallery: build then run the component gallery demo
run-gallery: gallery
	./$(BIN_DIR)/$(GALLERY)

## release-snapshot: build a local goreleaser snapshot (no publish)
release-snapshot:
	goreleaser release --snapshot --clean

## clean: remove build artifacts
clean:
	rm -rf $(BIN_DIR) dist
