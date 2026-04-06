BINARY   := nit
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS  := -s -w -X main.version=$(VERSION)
GOFLAGS  := -trimpath

.PHONY: build install test cover lint fmt vet check clean run release

## Build the binary
build:
	go build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $(BINARY) ./cmd/nit

## Build and install to ~/.local/bin (or PREFIX/bin)
PREFIX ?= $(HOME)/.local
install: build
	install -d $(PREFIX)/bin
	install -m 755 $(BINARY) $(PREFIX)/bin/$(BINARY)

## Run all tests
test:
	go test ./... -v -race -count=1

## Run tests with coverage
cover:
	go test ./... -coverprofile=coverage.out -race
	go tool cover -func=coverage.out

## Lint with golangci-lint
lint:
	golangci-lint run ./...

## Format code
fmt:
	gofumpt -w .
	goimports -w .

## Run go vet
vet:
	go vet ./...

## Run all checks (test + lint + vet)
check: test lint vet

## Remove build artifacts
clean:
	rm -f $(BINARY) coverage.out

## Build and run with args (usage: make run ARGS="--mode unstaged")
run: build
	./$(BINARY) $(ARGS)

## Release via goreleaser (requires GITHUB_TOKEN)
release:
	goreleaser release --clean
