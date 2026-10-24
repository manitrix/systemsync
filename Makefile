GO ?= go
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || echo unknown)
BUILD_DATE ?= $(shell git show -s --format=%cI HEAD 2>/dev/null || echo unknown)
LDFLAGS = -s -w -buildid= -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(BUILD_DATE)
.PHONY: build test integration race vet fmt check release clean
build:
	$(GO) build -trimpath -buildvcs=false -ldflags '$(LDFLAGS)' -o bin/ssync ./cmd/ssync
test:
	$(GO) test ./...
integration:
	$(GO) test -tags=integration ./test/...
race:
	CGO_ENABLED=1 $(GO) test -race ./...
vet:
	$(GO) vet ./...
fmt:
	gofmt -w cmd internal pkg test
check: test vet
	@test -z "$$(gofmt -l cmd internal pkg test)"
release:
	VERSION='$(VERSION)' COMMIT='$(COMMIT)' BUILD_DATE='$(BUILD_DATE)' sh scripts/release.sh
clean:
	rm -rf bin dist
