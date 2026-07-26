GO = CGO_ENABLED=0 go

.PHONY: all
all: test

.PHONY: test
test:
	$(GO) test -v ./...

.PHONY: format
format:
	$(GO) fmt ./...
