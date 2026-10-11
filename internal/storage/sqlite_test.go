package storage

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func newTestSQLite(t *testing.T, path string) *SQLiteStore {
	t.Helper()
	s, err := NewSQLite(path)
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestSQLiteStoreContract(t *testing.T) {
	runStoreContract(t, func(t *testing.T) Store {
		return newTestSQLite(t, filepath.Join(t.TempDir(), "test.db"))
	})
}

func TestSQLiteWAL(t *testing.T) {
	s := newTestSQLite(t, filepath.Join(t.TempDir(), "wal.db"))
	var mode string
	if err := s.db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatal(err)
	}
	if mode != "wal" {
		t.Fatalf("journal_mode = %q, want wal", mode)
	}
	var timeout int
	if err := s.db.QueryRow("PRAGMA busy_timeout").Scan(&timeout); err != nil {
		t.Fatal(err)
	}
	if timeout != int(busyTimeout.Milliseconds()) {
		t.Fatalf("busy_timeout = %d, want %d", timeout, busyTimeout.Milliseconds())
	}
}

func TestSQLitePersistsAcrossReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "sub dir", "persist.db")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	created := time.Date(2026, 10, 12, 10, 0, 0, 123456789, time.UTC)

	s, err := NewSQLite(path)
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	if err := s.Save(ctx, Link{Code: "keep", URL: "https://example.com/keep", CreatedAt: created}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	for i := 0; i < 2; i++ {
		if err := s.IncrementClicks(ctx, "keep"); err != nil {
			t.Fatalf("IncrementClicks: %v", err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	s2 := newTestSQLite(t, path)
	got, err := s2.Get(ctx, "keep")
	if err != nil {
		t.Fatalf("Get after reopen: %v", err)
	}
	if got.URL != "https://example.com/keep" || got.Clicks != 2 || !got.CreatedAt.Equal(created) {
		t.Fatalf("after reopen = %+v", got)
	}
}

// Два независимых пула к одному файлу — как два процесса: без busy_timeout
// часть UPDATE падала бы с SQLITE_BUSY.
func TestSQLiteConcurrentIncrementTwoHandles(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "conc.db")
	a := newTestSQLite(t, path)
	b := newTestSQLite(t, path)
	if err := a.Save(ctx, Link{Code: "c", URL: "https://example.com", CreatedAt: time.Now()}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	const workers, perWorker = 8, 50
	var wg sync.WaitGroup
	errs := make(chan error, workers*perWorker)
	for w := 0; w < workers; w++ {
		s := a
		if w%2 == 1 {
			s = b
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				if err := s.IncrementClicks(ctx, "c"); err != nil {
					errs <- err
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("IncrementClicks: %v", err)
	}
	got, err := b.Get(ctx, "c")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Clicks != workers*perWorker {
		t.Fatalf("Clicks = %d, want %d", got.Clicks, workers*perWorker)
	}
}

func TestNewSQLiteBadPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no-such-dir", "x.db")
	if s, err := NewSQLite(path); err == nil {
		s.Close()
		t.Fatal("NewSQLite succeeded for missing directory")
	}
}
