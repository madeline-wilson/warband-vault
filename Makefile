GO ?= $(shell if [ -x /opt/homebrew/opt/go@1.25/bin/go ]; then echo /opt/homebrew/opt/go@1.25/bin/go; else echo go; fi)
VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
CHANNEL ?= development
LDFLAGS := -s -w -X warband-vault/internal/buildinfo.Version=$(VERSION) -X warband-vault/internal/buildinfo.Commit=$(COMMIT) -X warband-vault/internal/buildinfo.BuildDate=$(BUILD_DATE) -X warband-vault/internal/buildinfo.Channel=$(CHANNEL)

.PHONY: fmt test race vet staticcheck vuln lint build smoke-test package release-local clean

fmt:
	$(GO)fmt -w cmd internal ui assets migrations

test:
	$(GO) test -count=1 ./...

race:
	$(GO) test -race -count=1 ./...

vet:
	$(GO) vet ./...

staticcheck:
	staticcheck ./...

vuln:
	govulncheck ./...

lint: vet staticcheck vuln

build:
	mkdir -p dist/bin
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o dist/bin/warband-vault ./cmd/warband-vault
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o dist/bin/warband-vault-launcher ./cmd/launcher
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o dist/bin/warband-vault-updater ./cmd/updater
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o dist/bin/manifest-tool ./cmd/manifest-tool
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o dist/bin/release-keygen ./cmd/release-keygen

smoke-test:
	$(GO) run ./cmd/warband-vault --smoke-test --data-dir "$$(mktemp -d)"

package:
	./scripts/package.sh

release-local: test build package

clean:
	rm -rf dist
