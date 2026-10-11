package storage

import "testing"

func TestMemoryStoreContract(t *testing.T) {
	runStoreContract(t, func(t *testing.T) Store {
		s := NewMemory()
		t.Cleanup(func() { _ = s.Close() })
		return s
	})
}
