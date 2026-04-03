.PHONY: build clean test build-linux build-darwin build-windows build-all

VERSION ?= dev
LDFLAGS = -ldflags "-X main.version=$(VERSION)"
BINS = fyvaultd fyvault fyvault-shim fyvault-health

build:
	go build $(LDFLAGS) -o bin/fyvaultd ./cmd/fyvaultd
	go build $(LDFLAGS) -o bin/fyvault ./cmd/fyvault
	go build $(LDFLAGS) -o bin/fyvault-shim ./cmd/fyvault-shim
	go build $(LDFLAGS) -o bin/fyvault-health ./cmd/fyvault-health

build-linux:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/linux-amd64/fyvaultd ./cmd/fyvaultd
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/linux-amd64/fyvault ./cmd/fyvault
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/linux-amd64/fyvault-shim ./cmd/fyvault-shim
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/linux-amd64/fyvault-health ./cmd/fyvault-health
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o bin/linux-arm64/fyvaultd ./cmd/fyvaultd
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o bin/linux-arm64/fyvault ./cmd/fyvault
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o bin/linux-arm64/fyvault-shim ./cmd/fyvault-shim
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o bin/linux-arm64/fyvault-health ./cmd/fyvault-health

build-darwin:
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o bin/darwin-amd64/fyvaultd ./cmd/fyvaultd
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o bin/darwin-amd64/fyvault ./cmd/fyvault
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o bin/darwin-amd64/fyvault-shim ./cmd/fyvault-shim
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o bin/darwin-amd64/fyvault-health ./cmd/fyvault-health
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o bin/darwin-arm64/fyvaultd ./cmd/fyvaultd
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o bin/darwin-arm64/fyvault ./cmd/fyvault
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o bin/darwin-arm64/fyvault-shim ./cmd/fyvault-shim
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o bin/darwin-arm64/fyvault-health ./cmd/fyvault-health

build-windows:
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/windows-amd64/fyvaultd.exe ./cmd/fyvaultd
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/windows-amd64/fyvault.exe ./cmd/fyvault
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/windows-amd64/fyvault-shim.exe ./cmd/fyvault-shim
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/windows-amd64/fyvault-health.exe ./cmd/fyvault-health
	GOOS=windows GOARCH=arm64 go build $(LDFLAGS) -o bin/windows-arm64/fyvaultd.exe ./cmd/fyvaultd
	GOOS=windows GOARCH=arm64 go build $(LDFLAGS) -o bin/windows-arm64/fyvault.exe ./cmd/fyvault
	GOOS=windows GOARCH=arm64 go build $(LDFLAGS) -o bin/windows-arm64/fyvault-shim.exe ./cmd/fyvault-shim
	GOOS=windows GOARCH=arm64 go build $(LDFLAGS) -o bin/windows-arm64/fyvault-health.exe ./cmd/fyvault-health

build-all: build-linux build-darwin build-windows

test:
	go test ./...

clean:
	rm -rf bin/
