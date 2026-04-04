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

# eBPF compilation (requires Linux with clang and kernel headers)
ebpf:
	clang -O2 -g -target bpf -D__TARGET_ARCH_x86 \
		-I/usr/include -I/usr/include/x86_64-linux-gnu \
		-c ebpf/tc_redirect.c -o ebpf/tc_redirect.o
	@echo "eBPF object compiled: ebpf/tc_redirect.o"

# eBPF for ARM64
ebpf-arm64:
	clang -O2 -g -target bpf -D__TARGET_ARCH_arm64 \
		-I/usr/include -I/usr/include/aarch64-linux-gnu \
		-c ebpf/tc_redirect.c -o ebpf/tc_redirect_arm64.o
	@echo "eBPF object compiled: ebpf/tc_redirect_arm64.o"

# Full Linux build with eBPF
build-linux-full: ebpf build-linux
	cp ebpf/tc_redirect.o bin/linux-amd64/
	@echo "Full Linux build with eBPF complete"
