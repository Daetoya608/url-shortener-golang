package storage

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// runStoreContract прогоняет общий набор тестов для любой реализации Store.
// newStore должен возвращать новое пустое хранилище.
func runStoreContract(t *testing.T, newStore func(t *testing.T) Store) {
	t.Helper()
	ctx := context.Background()
	created := time.Date(2026, 10, 12, 10, 0, 0, 0, time.UTC)

	t.Run("SaveGet", func(t *testing.T) {
		s := newStore(t)
		want := Link{Code: "abc1234", URL: "https://example.com/x", CreatedAt: created}
		if err := s.Save(ctx, want); err != nil {
			t.Fatalf("Save: %v", err)
		}
		got, err := s.Get(ctx, "abc1234")
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if got.Code != want.Code || got.URL != want.URL || got.Clicks != 0 || !got.CreatedAt.Equal(want.CreatedAt) {
			t.Fatalf("Get = %+v, want %+v", got, want)
		}
	})

	t.Run("DuplicateCode", func(t *testing.T) {
		s := newStore(t)
		l := Link{Code: "dup", URL: "https://a.example", CreatedAt: created}
		if err := s.Save(ctx, l); err != nil {
			t.Fatalf("Save: %v", err)
		}
		l.URL = "https://b.example"
		if err := s.Save(ctx, l); !errors.Is(err, ErrCodeExists) {
			t.Fatalf("second Save err = %v, want ErrCodeExists", err)
		}
		got, err := s.Get(ctx, "dup")
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if got.URL != "https://a.example" {
			t.Fatalf("URL overwritten: %q", got.URL)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		s := newStore(t)
		if _, err := s.Get(ctx, "missing"); !errors.Is(err, ErrNotFound) {
			t.Fatalf("Get err = %v, want ErrNotFound", err)
		}
		if err := s.IncrementClicks(ctx, "missing"); !errors.Is(err, ErrNotFound) {
			t.Fatalf("IncrementClicks err = %v, want ErrNotFound", err)
		}
	})

	t.Run("IncrementClicks", func(t *testing.T) {
		s := newStore(t)
		if err := s.Save(ctx, Link{Code: "inc", URL: "https://example.com", CreatedAt: created}); err != nil {
			t.Fatalf("Save: %v", err)
		}
		for i := 0; i < 3; i++ {
			if err := s.IncrementClicks(ctx, "inc"); err != nil {
				t.Fatalf("IncrementClicks: %v", err)
			}
		}
		got, err := s.Get(ctx, "inc")
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if got.Clicks != 3 {
			t.Fatalf("Clicks = %d, want 3", got.Clicks)
		}
	})

	t.Run("ConcurrentIncrementClicks", func(t *testing.T) {
		s := newStore(t)
		if err := s.Save(ctx, Link{Code: "conc", URL: "https://example.com", CreatedAt: created}); err != nil {
			t.Fatalf("Save: %v", err)
		}
		const workers, perWorker = 10, 50
		var wg sync.WaitGroup
		errs := make(chan error, workers*perWorker)
		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := 0; i < perWorker; i++ {
					if err := s.IncrementClicks(ctx, "conc"); err != nil {
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
		got, err := s.Get(ctx, "conc")
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if got.Clicks != workers*perWorker {
			t.Fatalf("Clicks = %d, want %d", got.Clicks, workers*perWorker)
		}
	})
}
