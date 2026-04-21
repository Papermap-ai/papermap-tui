APP := papermap
PKG := github.com/papermap/papermap-tui
BIN := ./bin/$(APP)

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
	-X main.version=$(VERSION) \
	-X main.commit=$(COMMIT) \
	-X main.date=$(DATE)

.PHONY: build run test fmt tidy lint hooks install release-snapshot clean help

help:
	@echo "Targets:"
	@echo "  build             Build $(APP) into $(BIN)"
	@echo "  run               Run papermap from source"
	@echo "  test              Run tests with race detector"
	@echo "  fmt               Format Go files (gofumpt + goimports)"
	@echo "  tidy              go mod tidy"
	@echo "  lint              Run golangci-lint"
	@echo "  hooks             Install repo git hooks (.githooks)"
	@echo "  install           go install $(PKG)/cmd/$(APP)"
	@echo "  release-snapshot  Run goreleaser in snapshot mode"
	@echo "  clean             Remove build artifacts"

build:
	@mkdir -p bin
	go build -trimpath -ldflags '$(LDFLAGS)' -o $(BIN) ./cmd/$(APP)

run:
	go run ./cmd/$(APP)

test:
	go test ./... -race -count=1

fmt:
	@command -v gofumpt >/dev/null 2>&1 || { echo "gofumpt not installed: go install mvdan.cc/gofumpt@latest"; exit 1; }
	@command -v goimports >/dev/null 2>&1 || { echo "goimports not installed: go install golang.org/x/tools/cmd/goimports@latest"; exit 1; }
	gofumpt -w cmd internal
	goimports -w -local $(PKG) cmd internal

tidy:
	go mod tidy

lint:
	@command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint not installed: https://golangci-lint.run/usage/install/"; exit 1; }
	golangci-lint run ./...

hooks:
	git config core.hooksPath .githooks
	@echo "Git hooks path set to .githooks"

install:
	go install -trimpath -ldflags '$(LDFLAGS)' $(PKG)/cmd/$(APP)

release-snapshot:
	@command -v goreleaser >/dev/null 2>&1 || { echo "goreleaser not installed: https://goreleaser.com/install/"; exit 1; }
	goreleaser release --snapshot --clean

clean:
	rm -rf bin dist
