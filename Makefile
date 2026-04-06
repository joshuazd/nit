BINARY   := nit
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS  := -s -w -X main.version=$(VERSION)
GOFLAGS  := -trimpath

.PHONY: build install test cover lint fmt vet check clean run release release-patch release-minor release-major release-dry

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

## Version auto-increment
LATEST_TAG := $(shell git describe --tags --abbrev=0 2>/dev/null || echo v0.0.0)
MAJOR := $(word 1,$(subst ., ,$(LATEST_TAG:v%=%)))
MINOR := $(word 2,$(subst ., ,$(LATEST_TAG:v%=%)))
PATCH := $(word 3,$(subst ., ,$(LATEST_TAG:v%=%)))

release-patch: v = $(MAJOR).$(MINOR).$(shell echo $$(($(PATCH)+1)))
release-minor: v = $(MAJOR).$(shell echo $$(($(MINOR)+1))).0
release-major: v = $(shell echo $$(($(MAJOR)+1))).0.0

## Release (usage: make release v=1.0.1, or make release-patch/minor/major)
release release-patch release-minor release-major: lint test
	@if [ -z "$(v)" ]; then echo "Usage: make release v=X.Y.Z (or use release-patch/minor/major)"; exit 1; fi
	@if [ -n "$$(git status --porcelain)" ]; then echo "Error: working tree is dirty — commit or stash first."; exit 1; fi
	@if git rev-parse "v$(v)" >/dev/null 2>&1; then echo "Error: tag v$(v) already exists."; exit 1; fi
	git tag "v$(v)"
	git push origin "v$(v)"
	gh release create "v$(v)" --title "v$(v)" --generate-notes
