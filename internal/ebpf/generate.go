//go:build ignore

package ebpf

// This file triggers bpf2go code generation.
// Run: go generate ./internal/ebpf/
// Requires: clang, llvm-strip, and linux kernel headers
//
// The generated files (bpf_bpfel.go, bpf_bpfeb.go, bpf_bpfel.o, bpf_bpfeb.o)
// contain the compiled eBPF bytecode embedded in Go source, so no external
// .o file needs to be shipped.

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang -target bpfel -type target_key -type target_value bpf ../../ebpf/tc_redirect.c -- -I/usr/include -I/usr/include/x86_64-linux-gnu -O2 -g -Wall
