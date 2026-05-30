APP      := pvectl
MODULE   := github.com/eltaline/pvectl
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT   := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE     := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS  := -s -w \
	-X '$(MODULE)/cmd.Version=$(VERSION)' \
	-X '$(MODULE)/cmd.Commit=$(COMMIT)' \
	-X '$(MODULE)/cmd.Date=$(DATE)'

.PHONY: build test vet lint clean

build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(APP) .

test:
	go test ./... -v

vet:
	go vet ./...

lint:
	golangci-lint run ./...

clean:
	rm -rf bin/
