VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
BINARY  := cowork-svc-linux
GOFLAGS := -trimpath
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build clean install uninstall lint test

build:
	CGO_ENABLED=0 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY) .

clean:
	rm -f $(BINARY)

install: build
	install -Dm755 $(BINARY) /usr/bin/$(BINARY)
	install -Dm644 dist/claude-cowork.service /usr/lib/systemd/user/claude-cowork.service

uninstall:
	rm -f /usr/bin/$(BINARY)
	rm -f /usr/lib/systemd/user/claude-cowork.service

lint:
	go vet ./...

test:
	go test ./...
