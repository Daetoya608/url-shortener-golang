package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"url-shortener/internal/service"
	"url-shortener/internal/storage"
)

const testBaseURL = "https://sho.rt"

var testNow = time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)

func newTestRouter(t *testing.T, opts ...service.Option) http.Handler {
	t.Helper()
	opts = append([]service.Option{service.WithClock(func() time.Time { return testNow })}, opts...)
	return NewRouter(service.New(storage.NewMemory(), opts...), testBaseURL)
}

func do(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return v
}

// assertError проверяет статус и что тело — JSON с непустым полем error.
func assertError(t *testing.T, rec *httptest.ResponseRecorder, status int) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d; body %s", rec.Code, status, rec.Body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
	if e := decode[errorResponse](t, rec); e.Error == "" {
		t.Fatalf("empty error field in %s", rec.Body)
	}
}

func TestShortenCreated(t *testing.T) {
	h := newTestRouter(t, service.WithGenerator(func() string { return "Abc1234" }))
	rec := do(t, h, http.MethodPost, "/api/shorten", `{"url":"https://example.com/a?b=1"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q", ct)
	}
	got := decode[shortenResponse](t, rec)
	want := shortenResponse{Code: "Abc1234", ShortURL: testBaseURL + "/Abc1234", URL: "https://example.com/a?b=1"}
	if got != want {
		t.Fatalf("response = %+v, want %+v", got, want)
	}
}

func TestShortenGeneratedCode(t *testing.T) {
	h := newTestRouter(t)
	rec := do(t, h, http.MethodPost, "/api/shorten", `{"url":"https://example.com"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	got := decode[shortenResponse](t, rec)
	if len(got.Code) != 7 || got.ShortURL != testBaseURL+"/"+got.Code {
		t.Fatalf("unexpected response %+v", got)
	}
}

func TestShortenWithAlias(t *testing.T) {
	h := newTestRouter(t)
	rec := do(t, h, http.MethodPost, "/api/shorten", `{"url":"https://example.com","alias":"my-link"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	got := decode[shortenResponse](t, rec)
	if got.Code != "my-link" || got.ShortURL != testBaseURL+"/my-link" {
		t.Fatalf("unexpected response %+v", got)
	}

	rec = do(t, h, http.MethodPost, "/api/shorten", `{"url":"https://other.example","alias":"my-link"}`)
	assertError(t, rec, http.StatusConflict)
}

func TestShortenBadRequest(t *testing.T) {
	cases := map[string]string{
		"invalid json":   `{"url":`,
		"not an object":  `["https://example.com"]`,
		"empty body":     ``,
		"trailing data":  `{"url":"https://example.com"} {}`,
		"invalid url":    `{"url":"ftp://example.com"}`,
		"missing url":    `{"alias":"abc"}`,
		"invalid alias":  `{"url":"https://example.com","alias":"a b"}`,
		"reserved alias": `{"url":"https://example.com","alias":"healthz"}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			assertError(t, do(t, newTestRouter(t), http.MethodPost, "/api/shorten", body), http.StatusBadRequest)
		})
	}
}

func TestShortenBodyTooLarge(t *testing.T) {
	body := `{"url":"https://example.com/` + strings.Repeat("a", MaxBodyBytes) + `"}`
	assertError(t, do(t, newTestRouter(t), http.MethodPost, "/api/shorten", body), http.StatusRequestEntityTooLarge)
}

func TestRedirectAndStats(t *testing.T) {
	h := newTestRouter(t)
	rec := do(t, h, http.MethodPost, "/api/shorten", `{"url":"https://example.com/target","alias":"go-here"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: status = %d, body %s", rec.Code, rec.Body)
	}

	rec = do(t, h, http.MethodGet, "/api/links/go-here", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("stats: status = %d, body %s", rec.Code, rec.Body)
	}
	if st := decode[statsResponse](t, rec); st.Clicks != 0 {
		t.Fatalf("clicks before redirect = %d", st.Clicks)
	}

	for range 2 {
		rec = do(t, h, http.MethodGet, "/go-here", "")
		if rec.Code != http.StatusFound {
			t.Fatalf("redirect: status = %d, body %s", rec.Code, rec.Body)
		}
		if loc := rec.Header().Get("Location"); loc != "https://example.com/target" {
			t.Fatalf("Location = %q", loc)
		}
	}

	rec = do(t, h, http.MethodGet, "/api/links/go-here", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("stats: status = %d, body %s", rec.Code, rec.Body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q", ct)
	}
	got := decode[statsResponse](t, rec)
	want := statsResponse{Code: "go-here", URL: "https://example.com/target", CreatedAt: testNow, Clicks: 2}
	if !got.CreatedAt.Equal(want.CreatedAt) || got.Code != want.Code || got.URL != want.URL || got.Clicks != want.Clicks {
		t.Fatalf("stats = %+v, want %+v", got, want)
	}
	if !strings.Contains(rec.Body.String(), `"created_at":"2026-10-12T12:00:00Z"`) {
		t.Fatalf("created_at not in RFC 3339: %s", rec.Body)
	}
}

func TestNotFound(t *testing.T) {
	h := newTestRouter(t)
	for _, path := range []string{"/unknown", "/api/links/unknown", "/", "/a/b/c"} {
		t.Run(path, func(t *testing.T) {
			assertError(t, do(t, h, http.MethodGet, path, ""), http.StatusNotFound)
		})
	}
}

func TestHealthz(t *testing.T) {
	rec := do(t, newTestRouter(t), http.MethodGet, "/healthz", "")
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("healthz = %d %q", rec.Code, rec.Body)
	}
}
