GO = CGO_ENABLED=0 go
TARGETS = \
  darwin/amd64 \
  darwin/arm64 \
  linux/amd64 \
  linux/arm64 \
  linux/riscv64 \
  windows/amd64 \
  windows/arm64

.PHONY: all
all: build-all test

.PHONY: build-all
build-all:
	for target in $(TARGETS); do \
	  GOOS="$${target%/*}" GOARCH="$${target#*/}" $(GO) build ./...; \
	done

.PHONY: test
test:
	$(GO) test -v ./...

.PHONY: format
format:
	$(GO) fmt ./...
