package keyring

import "testing"

func TestKeyringStoreAndRead(t *testing.T) {
	kr, err := New("test")
	if err != nil {
		t.Fatal(err)
	}

	if err := kr.Store("MY_SECRET", []byte("secret-value-123")); err != nil {
		t.Fatal(err)
	}

	val, err := kr.Read("MY_SECRET")
	if err != nil {
		t.Fatal(err)
	}
	if string(val) != "secret-value-123" {
		t.Errorf("Read() = %q, want %q", string(val), "secret-value-123")
	}
}

func TestKeyringReadMissing(t *testing.T) {
	kr, err := New("test")
	if err != nil {
		t.Fatal(err)
	}

	_, err = kr.Read("NONEXISTENT")
	if err == nil {
		t.Error("expected error for missing key, got nil")
	}
}

func TestKeyringOverwrite(t *testing.T) {
	kr, _ := New("test")

	kr.Store("KEY", []byte("value1"))
	kr.Store("KEY", []byte("value2"))

	val, err := kr.Read("KEY")
	if err != nil {
		t.Fatal(err)
	}
	if string(val) != "value2" {
		t.Errorf("Read() = %q after overwrite, want %q", string(val), "value2")
	}
}

func TestKeyringDelete(t *testing.T) {
	kr, _ := New("test")

	kr.Store("DELETE_ME", []byte("value"))
	kr.Delete("DELETE_ME")

	_, err := kr.Read("DELETE_ME")
	if err == nil {
		t.Error("expected error after delete, got nil")
	}
}

func TestKeyringDeleteNonExistent(t *testing.T) {
	kr, _ := New("test")

	// Should not panic or error
	err := kr.Delete("DOES_NOT_EXIST")
	if err != nil {
		t.Errorf("Delete non-existent key returned error: %v", err)
	}
}

func TestKeyringFlushAll(t *testing.T) {
	kr, _ := New("test")

	kr.Store("KEY1", []byte("v1"))
	kr.Store("KEY2", []byte("v2"))
	kr.Store("KEY3", []byte("v3"))

	kr.FlushAll()

	if kr.Count() != 0 {
		t.Errorf("Count() = %d after FlushAll, want 0", kr.Count())
	}

	_, err := kr.Read("KEY1")
	if err == nil {
		t.Error("expected error reading KEY1 after FlushAll")
	}
}

func TestKeyringCount(t *testing.T) {
	kr, _ := New("test")

	if kr.Count() != 0 {
		t.Errorf("Count() = %d for empty keyring, want 0", kr.Count())
	}

	kr.Store("A", []byte("1"))
	kr.Store("B", []byte("2"))

	if kr.Count() != 2 {
		t.Errorf("Count() = %d after storing 2 keys, want 2", kr.Count())
	}
}

func TestKeyringNamespacing(t *testing.T) {
	kr1, _ := New("ns1")
	kr2, _ := New("ns2")

	kr1.Store("KEY", []byte("value1"))
	kr2.Store("KEY", []byte("value2"))

	v1, err := kr1.Read("KEY")
	if err != nil {
		t.Fatal(err)
	}
	v2, err := kr2.Read("KEY")
	if err != nil {
		t.Fatal(err)
	}

	if string(v1) != "value1" {
		t.Errorf("ns1 KEY = %q, want %q", string(v1), "value1")
	}
	if string(v2) != "value2" {
		t.Errorf("ns2 KEY = %q, want %q", string(v2), "value2")
	}
}

func TestKeyringStoreIsolation(t *testing.T) {
	// Verify that stored values are copies, not references
	kr, _ := New("test")
	original := []byte("original-value")

	kr.Store("COPY_TEST", original)

	// Mutate the original slice
	original[0] = 'X'

	val, _ := kr.Read("COPY_TEST")
	if string(val) != "original-value" {
		t.Errorf("Store did not copy value; Read() = %q", string(val))
	}
}

func TestKeyringBinaryValues(t *testing.T) {
	kr, _ := New("test")

	binaryData := []byte{0x00, 0xFF, 0x01, 0xFE, 0x00, 0x00}
	kr.Store("BINARY", binaryData)

	val, err := kr.Read("BINARY")
	if err != nil {
		t.Fatal(err)
	}
	if len(val) != len(binaryData) {
		t.Errorf("binary length = %d, want %d", len(val), len(binaryData))
	}
	for i := range binaryData {
		if val[i] != binaryData[i] {
			t.Errorf("binary byte %d = %x, want %x", i, val[i], binaryData[i])
		}
	}
}
