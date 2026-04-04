//go:build windows

package keyring

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// Keyring uses an AES-encrypted file store on Windows.
// A future version can integrate with Windows Credential Manager for
// individual entries; the file store handles bulk secret storage well.
type Keyring struct {
	namespace string
	mu        sync.RWMutex
	store     map[string][]byte
	path      string
}

// New creates a file-backed encrypted Keyring on Windows.
func New(namespace string) (*Keyring, error) {
	dir := filepath.Join(os.Getenv("APPDATA"), "fyvault")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create data dir: %w", err)
	}

	k := &Keyring{
		namespace: namespace,
		store:     make(map[string][]byte),
		path:      filepath.Join(dir, "keyring.enc"),
	}
	k.load()
	return k, nil
}

func (k *Keyring) keyName(name string) string {
	return k.namespace + ":" + name
}

func (k *Keyring) encKey() []byte {
	h := sha256.Sum256([]byte("fyvault-windows-keyring:" + k.namespace))
	return h[:]
}

// Store saves a secret and persists to the encrypted file.
func (k *Keyring) Store(name string, value []byte) error {
	k.mu.Lock()
	k.store[k.keyName(name)] = append([]byte{}, value...)
	err := k.persist()
	k.mu.Unlock()
	return err
}

// Read retrieves a secret from the store.
func (k *Keyring) Read(name string) ([]byte, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	v, ok := k.store[k.keyName(name)]
	if !ok {
		return nil, fmt.Errorf("key not found: %s", name)
	}
	return v, nil
}

// Delete removes a secret and persists.
func (k *Keyring) Delete(name string) error {
	k.mu.Lock()
	delete(k.store, k.keyName(name))
	err := k.persist()
	k.mu.Unlock()
	return err
}

// Count returns the number of stored secrets.
func (k *Keyring) Count() int {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return len(k.store)
}

// FlushAll removes all secrets and deletes the file.
func (k *Keyring) FlushAll() error {
	k.mu.Lock()
	k.store = make(map[string][]byte)
	os.Remove(k.path)
	k.mu.Unlock()
	return nil
}

func (k *Keyring) persist() error {
	plain, err := json.Marshal(k.store)
	if err != nil {
		return err
	}

	block, err := aes.NewCipher(k.encKey())
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}

	ciphertext := gcm.Seal(nonce, nonce, plain, nil)
	return os.WriteFile(k.path, ciphertext, 0600)
}

func (k *Keyring) load() {
	data, err := os.ReadFile(k.path)
	if err != nil {
		return
	}

	block, err := aes.NewCipher(k.encKey())
	if err != nil {
		return
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return
	}

	plaintext, err := gcm.Open(nil, data[:nonceSize], data[nonceSize:], nil)
	if err != nil {
		return
	}

	json.Unmarshal(plaintext, &k.store)
}
