//go:build !linux

package enclave

func isNitroEnclave() bool { return false }
func isAMDSEV() bool       { return false }
func isIntelTDX() bool     { return false }
