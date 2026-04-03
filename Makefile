.PHONY: build clean test

VERSION ?= dev
LDFLAGS = -ldflags "-X main.version=$(VERSION)"

build:
	go build $(LDFLAGS) -o bin/fyvaultd ./cmd/fyvaultd
	go build $(LDFLAGS) -o bin/fyvault ./cmd/fyvault
	go build $(LDFLAGS) -o bin/fyvault-shim ./cmd/fyvault-shim
	go build $(LDFLAGS) -o bin/fyvault-health ./cmd/fyvault-health

test:
	go test ./...

clean:
	rm -rf bin/
