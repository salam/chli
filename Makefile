BINARY  := chli
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
MODULE  := $(shell head -1 go.mod | awk '{print $$2}')
LDFLAGS := -s -w \
	-X '$(MODULE)/cmd.version=$(VERSION)' \
	-X '$(MODULE)/cmd.commit=$(COMMIT)' \
	-X '$(MODULE)/cmd.buildDate=$(DATE)'

.PHONY: build clean all test lint

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

clean:
	rm -f $(BINARY)
	rm -rf dist

test:
	go test ./... -v

lint:
	go vet ./...

all: clean
	GOOS=darwin  GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-darwin-amd64 .
	GOOS=darwin  GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-darwin-arm64 .
	GOOS=linux   GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-linux-amd64 .
	GOOS=linux   GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-linux-arm64 .
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-windows-amd64.exe .
