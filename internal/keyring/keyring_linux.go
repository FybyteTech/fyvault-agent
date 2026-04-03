//go:build linux

package keyring

import (
	"fmt"

	"golang.org/x/sys/unix"
)

const keyType = "user"

// Keyring wraps the Linux kernel keyring for secret storage.
type Keyring struct {
	sessionID int32
	namespace string
}

// New creates a Keyring backed by the Linux kernel session keyring.
func New(namespace string) (*Keyring, error) {
	id, err := unix.KeyctlGetKeyringID(unix.KEY_SPEC_SESSION_KEYRING, true)
	if err != nil {
		return nil, fmt.Errorf("failed to get session keyring: %w", err)
	}
	return &Keyring{sessionID: int32(id), namespace: namespace}, nil
}

func (k *Keyring) keyName(name string) string {
	return k.namespace + ":" + name
}

// Store adds or updates a key in the session keyring.
func (k *Keyring) Store(name string, value []byte) error {
	_, err := unix.AddKey(keyType, k.keyName(name), value, int(k.sessionID))
	return err
}

// Read retrieves a key's value from the session keyring.
func (k *Keyring) Read(name string) ([]byte, error) {
	id, err := unix.KeyctlSearch(int(k.sessionID), keyType, k.keyName(name), 0)
	if err != nil {
		return nil, fmt.Errorf("key not found: %s: %w", name, err)
	}
	buf := make([]byte, 4096)
	n, err := unix.KeyctlBuffer(unix.KEYCTL_READ, id, buf, 4096)
	if err != nil {
		return nil, fmt.Errorf("failed to read key: %w", err)
	}
	return buf[:n], nil
}

// Delete revokes a key from the session keyring.
func (k *Keyring) Delete(name string) error {
	id, err := unix.KeyctlSearch(int(k.sessionID), keyType, k.keyName(name), 0)
	if err != nil {
		return nil // key doesn't exist
	}
	_, err = unix.KeyctlInt(unix.KEYCTL_REVOKE, id, 0, 0, 0)
	return err
}

// Count returns the number of keys (stub; real enumeration is complex).
func (k *Keyring) Count() int {
	return 0
}

// FlushAll revokes the entire session keyring.
func (k *Keyring) FlushAll() error {
	_, err := unix.KeyctlInt(unix.KEYCTL_REVOKE, int(k.sessionID), 0, 0, 0)
	return err
}
