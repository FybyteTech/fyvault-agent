//go:build linux

package enclave

import "os"

func isNitroEnclave() bool {
	// Nitro Enclaves expose /dev/nsm (Nitro Security Module)
	_, err := os.Stat("/dev/nsm")
	return err == nil
}

func isAMDSEV() bool {
	// AMD SEV exposes /dev/sev-guest or sysfs flag
	if _, err := os.Stat("/dev/sev-guest"); err == nil {
		return true
	}
	// Check sysfs
	data, err := os.ReadFile("/sys/module/kvm_amd/parameters/sev")
	if err == nil && len(data) > 0 && (data[0] == 'Y' || data[0] == '1') {
		return true
	}
	return false
}

func isIntelTDX() bool {
	// Intel TDX exposes /dev/tdx_guest
	_, err := os.Stat("/dev/tdx_guest")
	return err == nil
}
