package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/fybyte/fyvault-agent/internal/keyring"
)

// CredentialProcess is the AWS credential_process JSON output format.
type CredentialProcess struct {
	Version         int    `json:"Version"`
	AccessKeyId     string `json:"AccessKeyId"`
	SecretAccessKey string `json:"SecretAccessKey"`
	SessionToken    string `json:"SessionToken,omitempty"`
	Expiration      string `json:"Expiration,omitempty"`
}

func main() {
	kr, err := keyring.New("fyvault")
	if err != nil {
		fmt.Fprintf(os.Stderr, "keyring error: %v\n", err)
		os.Exit(1)
	}

	accessKeyID, err := kr.Read("AWS_ACCESS_KEY_ID")
	if err != nil {
		fmt.Fprintf(os.Stderr, "AWS_ACCESS_KEY_ID not found in keyring: %v\n", err)
		os.Exit(1)
	}

	secretKey, err := kr.Read("AWS_SECRET_ACCESS_KEY")
	if err != nil {
		fmt.Fprintf(os.Stderr, "AWS_SECRET_ACCESS_KEY not found in keyring: %v\n", err)
		os.Exit(1)
	}

	cred := CredentialProcess{
		Version:         1,
		AccessKeyId:     string(accessKeyID),
		SecretAccessKey: string(secretKey),
	}

	// Optional session token.
	if token, err := kr.Read("AWS_SESSION_TOKEN"); err == nil {
		cred.SessionToken = string(token)
	}

	// Set expiration to 1 hour from now (forces periodic refresh).
	cred.Expiration = time.Now().Add(1 * time.Hour).UTC().Format(time.RFC3339)

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(cred); err != nil {
		fmt.Fprintf(os.Stderr, "JSON encode error: %v\n", err)
		os.Exit(1)
	}
}
