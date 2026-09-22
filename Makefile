MODULE  := github.com/forlwow/labherd
BIN     := bin
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X $(MODULE)/internal/common/version.Version=$(VERSION)

.PHONY: all fmt lint test build clean

all: fmt lint test build

fmt:
	golangci-lint fmt

lint:
	golangci-lint run

test:
	go test -race -cover ./...

build:
	go build -ldflags "$(LDFLAGS)" -o $(BIN)/labherd-agent ./cmd/labherd-agent
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BIN)/labherd-controller ./cmd/labherd-controller

clean:
	rm -rf $(BIN) coverage.out
