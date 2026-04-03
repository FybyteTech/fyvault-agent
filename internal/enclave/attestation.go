package enclave

// Attestation holds a hardware-signed attestation document.
type Attestation struct {
	Platform string `json:"platform"` // "aws_nitro", "amd_sev", "intel_tdx"
	Document string `json:"document"` // base64-encoded attestation document
	Nonce    string `json:"nonce"`    // challenge nonce from server
}

// GenerateAttestation creates a hardware attestation document.
// On non-confidential hardware, returns nil (no attestation available).
func GenerateAttestation(detection Detection, nonce string) *Attestation {
	if detection.Level == LevelStandard {
		return nil
	}

	// For now, return a placeholder. Real implementation requires:
	// - Nitro: NSM ioctl to /dev/nsm -> signed attestation doc
	// - SEV: ioctl to /dev/sev-guest -> SNP report
	// - TDX: ioctl to /dev/tdx_guest -> TDX report
	return &Attestation{
		Platform: detection.Platform,
		Document: "placeholder-attestation-doc",
		Nonce:    nonce,
	}
}
