package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"url-shortener/internal/shortcode"
	"url-shortener/internal/storage"
)

// seqGen возвращает коды из списка по очереди и считает вызовы.
type seqGen struct {
	codes []string
	calls int
}

func (g *seqGen) next() string {
	c := g.codes[g.calls%len(g.codes)]
	g.calls++
	return c
}

func TestShortenGeneratesCode(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	svc := New(storage.NewMemory(), WithClock(func() time.Time { return now }))

	l, err := svc.Shorten(ctx, "https://example.com/path?q=1", "")
	if err != nil {
		t.Fatalf("Shorten: %v", err)
	}
	if len(l.Code) != shortcode.Length {
		t.Fatalf("code %q has length %d", l.Code, len(l.Code))
	}
	if l.URL != "https://example.com/path?q=1" || !l.CreatedAt.Equal(now) || l.Clicks != 0 {
		t.Fatalf("unexpected link %+v", l)
	}
	got, err := svc.Stats(ctx, l.Code)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if got.URL != l.URL {
		t.Fatalf("Stats URL = %q, want %q", got.URL, l.URL)
	}
}

func TestShortenInvalidURL(t *testing.T) {
	svc := New(storage.NewMemory())
	cases := map[string]string{
		"empty":                "",
		"ftp":                  "ftp://example.com/file",
		"no scheme":            "example.com",
		"empty host":           "http://",
		"empty host with path": "https:///path",
		"garbage":              "not a url at all",
		"bad escape":           "http://exa mple.com/%zz",
		"javascript":           "javascript:alert(1)",
		"too long":             "https://example.com/" + strings.Repeat("a", MaxURLLength),
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := svc.Shorten(context.Background(), raw, ""); !errors.Is(err, ErrInvalidURL) {
				t.Fatalf("Shorten(%q) err = %v, want ErrInvalidURL", raw, err)
			}
		})
	}
}

func TestShortenMaxLengthURLAccepted(t *testing.T) {
	svc := New(storage.NewMemory())
	prefix := "https://example.com/"
	raw := prefix + strings.Repeat("a", MaxURLLength-len(prefix))
	if _, err := svc.Shorten(context.Background(), raw, ""); err != nil {
		t.Fatalf("Shorten of %d-byte URL: %v", len(raw), err)
	}
}

func TestShortenAlias(t *testing.T) {
	ctx := context.Background()
	svc := New(storage.NewMemory())

	l, err := svc.Shorten(ctx, "https://example.com", "my-link")
	if err != nil {
		t.Fatalf("Shorten: %v", err)
	}
	if l.Code != "my-link" {
		t.Fatalf("code = %q, want my-link", l.Code)
	}

	if _, err := svc.Shorten(ctx, "https://other.example", "my-link"); !errors.Is(err, ErrAliasTaken) {
		t.Fatalf("taken alias err = %v, want ErrAliasTaken", err)
	}
}

func TestShortenInvalidAlias(t *testing.T) {
	svc := New(storage.NewMemory())
	for _, alias := range []string{"api", "healthz", "ab", "bad alias", "a/b", strings.Repeat("x", 33)} {
		if _, err := svc.Shorten(context.Background(), "https://example.com", alias); !errors.Is(err, ErrInvalidAlias) {
			t.Errorf("alias %q err = %v, want ErrInvalidAlias", alias, err)
		}
	}
}

func TestShortenInvalidURLCheckedBeforeAlias(t *testing.T) {
	svc := New(storage.NewMemory())
	if _, err := svc.Shorten(context.Background(), "ftp://x", "api"); !errors.Is(err, ErrInvalidURL) {
		t.Fatalf("err = %v, want ErrInvalidURL", err)
	}
}

func TestShortenRetriesOnCollision(t *testing.T) {
	ctx := context.Background()
	store := storage.NewMemory()
	gen := &seqGen{codes: []string{"AAAAAAA", "AAAAAAA", "AAAAAAA", "BBBBBBB"}}
	svc := New(store, WithGenerator(gen.next))

	first, err := svc.Shorten(ctx, "https://one.example", "")
	if err != nil || first.Code != "AAAAAAA" {
		t.Fatalf("first Shorten = %+v, %v", first, err)
	}
	second, err := svc.Shorten(ctx, "https://two.example", "")
	if err != nil {
		t.Fatalf("second Shorten: %v", err)
	}
	if second.Code != "BBBBBBB" {
		t.Fatalf("second code = %q, want BBBBBBB", second.Code)
	}
	if gen.calls != 4 {
		t.Fatalf("generator calls = %d, want 4", gen.calls)
	}
}

func TestShortenSkipsReservedGeneratedCode(t *testing.T) {
	gen := &seqGen{codes: []string{"healthz", "CCCCCCC"}}
	svc := New(storage.NewMemory(), WithGenerator(gen.next))
	l, err := svc.Shorten(context.Background(), "https://example.com", "")
	if err != nil {
		t.Fatalf("Shorten: %v", err)
	}
	if l.Code != "CCCCCCC" {
		t.Fatalf("code = %q, want CCCCCCC", l.Code)
	}
}

func TestShortenGivesUpAfterMaxAttempts(t *testing.T) {
	ctx := context.Background()
	store := storage.NewMemory()
	gen := &seqGen{codes: []string{"AAAAAAA"}}
	svc := New(store, WithGenerator(gen.next))

	if _, err := svc.Shorten(ctx, "https://one.example", ""); err != nil {
		t.Fatalf("first Shorten: %v", err)
	}
	gen.calls = 0
	_, err := svc.Shorten(ctx, "https://two.example", "")
	if err == nil {
		t.Fatal("expected error when all codes collide")
	}
	if errors.Is(err, ErrInvalidURL) || errors.Is(err, ErrAliasTaken) {
		t.Fatalf("unexpected error kind: %v", err)
	}
	if gen.calls != maxGenerateAttempts {
		t.Fatalf("generator calls = %d, want %d", gen.calls, maxGenerateAttempts)
	}
}

func TestResolveIncrementsClicks(t *testing.T) {
	ctx := context.Background()
	svc := New(storage.NewMemory())
	l, err := svc.Shorten(ctx, "https://example.com", "")
	if err != nil {
		t.Fatalf("Shorten: %v", err)
	}
	for i := int64(1); i <= 3; i++ {
		got, err := svc.Resolve(ctx, l.Code)
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if got.URL != "https://example.com" || got.Clicks != i {
			t.Fatalf("Resolve #%d = %+v", i, got)
		}
	}
	st, err := svc.Stats(ctx, l.Code)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if st.Clicks != 3 {
		t.Fatalf("Stats clicks = %d, want 3", st.Clicks)
	}
}

func TestNotFound(t *testing.T) {
	ctx := context.Background()
	svc := New(storage.NewMemory())
	if _, err := svc.Resolve(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Resolve err = %v, want ErrNotFound", err)
	}
	if _, err := svc.Stats(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Stats err = %v, want ErrNotFound", err)
	}
}
