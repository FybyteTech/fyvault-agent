//go:build darwin

package keyring

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

// Keyring wraps the macOS Keychain via the `security` CLI tool.
type Keyring struct {
	namespace string
	mu        sync.RWMutex
	cache     map[string][]byte
}

const serviceName = "com.fybyte.fyvault"

// New creates a Keyring backed by the macOS Keychain.
func New(namespace string) (*Keyring, error) {
	return &Keyring{
		namespace: namespace,
		cache:     make(map[string][]byte),
	}, nil
}

func (k *Keyring) account(name string) string {
	return k.namespace + ":" + name
}

// Store adds or updates a secret in the macOS Keychain.
func (k *Keyring) Store(name string, value []byte) error {
	acct := k.account(name)

	// Delete any existing entry first (update = delete + add).
	exec.Command("security", "delete-generic-password",
		"-s", serviceName, "-a", acct).Run()

	cmd := exec.Command("security", "add-generic-password",
		"-s", serviceName, "-a", acct,
		"-w", string(value),
		"-U")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("keychain store failed: %w: %s", err, string(out))
	}

	k.mu.Lock()
	k.cache[acct] = append([]byte{}, value...)
	k.mu.Unlock()
	return nil
}

// Read retrieves a secret from the macOS Keychain.
func (k *Keyring) Read(name string) ([]byte, error) {
	acct := k.account(name)

	k.mu.RLock()
	if v, ok := k.cache[acct]; ok {
		k.mu.RUnlock()
		return v, nil
	}
	k.mu.RUnlock()

	cmd := exec.Command("security", "find-generic-password",
		"-s", serviceName, "-a", acct, "-w")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("key not found: %s", name)
	}

	val := []byte(strings.TrimRight(string(out), "\n"))
	k.mu.Lock()
	k.cache[acct] = val
	k.mu.Unlock()
	return val, nil
}

// Delete removes a secret from the macOS Keychain.
func (k *Keyring) Delete(name string) error {
	acct := k.account(name)
	exec.Command("security", "delete-generic-password",
		"-s", serviceName, "-a", acct).Run()

	k.mu.Lock()
	delete(k.cache, acct)
	k.mu.Unlock()
	return nil
}

// Count returns the number of cached secrets.
func (k *Keyring) Count() int {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return len(k.cache)
}

// FlushAll removes all fyvault entries from the Keychain.
func (k *Keyring) FlushAll() error {
	k.mu.Lock()
	for acct := range k.cache {
		exec.Command("security", "delete-generic-password",
			"-s", serviceName, "-a", acct).Run()
	}
	k.cache = make(map[string][]byte)
	k.mu.Unlock()
	return nil
}
