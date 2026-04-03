package enclave

// SecurityLevel represents the confidential computing tier of the runtime.
type SecurityLevel string

const (
	LevelStandard       SecurityLevel = "standard"
	LevelConfidentialVM SecurityLevel = "confidential_vm"
	LevelNitroEnclave   SecurityLevel = "nitro_enclave"
)

// Detection holds the result of runtime confidential computing detection.
type Detection struct {
	Level    SecurityLevel
	Platform string // "aws_nitro", "amd_sev", "intel_tdx", "none"
	Details  map[string]string
}

// Detect checks the runtime environment for confidential computing hardware.
// Runs automatically at boot — no customer configuration needed.
func Detect() Detection {
	// Check Nitro Enclave first (most specific)
	if isNitroEnclave() {
		return Detection{Level: LevelNitroEnclave, Platform: "aws_nitro", Details: map[string]string{"device": "/dev/nsm"}}
	}

	// Check AMD SEV-SNP
	if isAMDSEV() {
		return Detection{Level: LevelConfidentialVM, Platform: "amd_sev", Details: map[string]string{"sev": "enabled"}}
	}

	// Check Intel TDX
	if isIntelTDX() {
		return Detection{Level: LevelConfidentialVM, Platform: "intel_tdx", Details: map[string]string{"tdx": "enabled"}}
	}

	return Detection{Level: LevelStandard, Platform: "none", Details: nil}
}
