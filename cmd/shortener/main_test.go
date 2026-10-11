package main

import (
	"os"
	"path/filepath"
	"testing"

	"url-shortener/internal/storage"
)

func TestOpenStoreMemory(t *testing.T) {
	s, _, err := openStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, ok := s.(*storage.Memory); !ok {
		t.Fatalf("store = %T, want *storage.Memory", s)
	}
}

func TestOpenStoreSQLite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shortener.db")
	s, _, err := openStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, ok := s.(*storage.SQLiteStore); !ok {
		t.Fatalf("store = %T, want *storage.SQLiteStore", s)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("db file not created: %v", err)
	}
}
