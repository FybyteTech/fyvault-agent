//go:build !linux && !darwin && !windows

package keyring

import "fmt"

// Keyring is an in-memory fallback for unsupported platforms.
type Keyring struct {
	namespace string
	store     map[string][]byte
}

// New creates an in-memory keyring for development on non-Linux systems.
func New(namespace string) (*Keyring, error) {
	return &Keyring{namespace: namespace, store: make(map[string][]byte)}, nil
}

func (k *Keyring) keyName(name string) string {
	return k.namespace + ":" + name
}

// Store saves a secret in the in-memory map.
func (k *Keyring) Store(name string, value []byte) error {
	k.store[k.keyName(name)] = append([]byte{}, value...)
	return nil
}

// Read retrieves a secret from the in-memory map.
func (k *Keyring) Read(name string) ([]byte, error) {
	v, ok := k.store[k.keyName(name)]
	if !ok {
		return nil, fmt.Errorf("key not found: %s", name)
	}
	return v, nil
}

// Delete removes a secret from the in-memory map.
func (k *Keyring) Delete(name string) error {
	delete(k.store, k.keyName(name))
	return nil
}

// Count returns the number of stored secrets.
func (k *Keyring) Count() int {
	return len(k.store)
}

// FlushAll removes all secrets from the in-memory map.
func (k *Keyring) FlushAll() error {
	k.store = make(map[string][]byte)
	return nil
}
