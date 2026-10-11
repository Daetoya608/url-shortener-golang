// Package httpapi — HTTP-хендлеры и роутинг сервиса коротких ссылок.
package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"url-shortener/internal/service"
	"url-shortener/internal/storage"
)

// MaxBodyBytes — максимальный размер тела запроса.
const MaxBodyBytes = 1 << 20

type shortenRequest struct {
	URL   string `json:"url"`
	Alias string `json:"alias,omitempty"`
}

type shortenResponse struct {
	Code     string `json:"code"`
	ShortURL string `json:"short_url"`
	URL      string `json:"url"`
}

type statsResponse struct {
	Code      string    `json:"code"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"created_at"`
	Clicks    int64     `json:"clicks"`
}

type errorResponse struct {
	Error string `json:"error"`
}

type api struct {
	svc     *service.Service
	baseURL string
	log     *slog.Logger
}

// NewRouter собирает HTTP-роутер поверх сервиса. baseURL — префикс short_url
// без завершающего "/".
func NewRouter(svc *service.Service, baseURL string) http.Handler {
	a := &api{svc: svc, baseURL: baseURL, log: slog.Default()}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/shorten", a.shorten)
	mux.HandleFunc("GET /api/links/{code}", a.stats)
	mux.HandleFunc("GET /healthz", a.healthz)
	mux.HandleFunc("GET /{code}", a.redirect)
	mux.HandleFunc("/", a.notFound)
	return mux
}

func (a *api) shorten(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)
	var req shortenRequest
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&req); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if _, err := dec.Token(); err != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid json: unexpected data after object")
		return
	}

	link, err := a.svc.Shorten(r.Context(), req.URL, req.Alias)
	switch {
	case errors.Is(err, service.ErrInvalidURL), errors.Is(err, service.ErrInvalidAlias):
		writeError(w, http.StatusBadRequest, err.Error())
		return
	case errors.Is(err, service.ErrAliasTaken):
		writeError(w, http.StatusConflict, err.Error())
		return
	case err != nil:
		a.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, shortenResponse{
		Code:     link.Code,
		ShortURL: a.baseURL + "/" + link.Code,
		URL:      link.URL,
	})
}

func (a *api) redirect(w http.ResponseWriter, r *http.Request) {
	link, err := a.svc.Resolve(r.Context(), r.PathValue("code"))
	if !a.handleLookupError(w, r, err) {
		return
	}
	// Без кэширования, иначе браузер не придёт повторно и клики не посчитаются.
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, link.URL, http.StatusFound)
}

func (a *api) stats(w http.ResponseWriter, r *http.Request) {
	link, err := a.svc.Stats(r.Context(), r.PathValue("code"))
	if !a.handleLookupError(w, r, err) {
		return
	}
	writeJSON(w, http.StatusOK, toStats(link))
}

func (a *api) healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = io.WriteString(w, "ok")
}

func (a *api) notFound(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotFound, "not found")
}

// handleLookupError пишет ответ на ошибку поиска ссылки; возвращает true, если ошибки нет.
func (a *api) handleLookupError(w http.ResponseWriter, r *http.Request, err error) bool {
	switch {
	case err == nil:
		return true
	case errors.Is(err, service.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	default:
		a.internalError(w, r, err)
	}
	return false
}

func (a *api) internalError(w http.ResponseWriter, r *http.Request, err error) {
	a.log.Error("request failed", "method", r.Method, "path", r.URL.Path, "err", err)
	writeError(w, http.StatusInternalServerError, "internal error")
}

func toStats(l storage.Link) statsResponse {
	return statsResponse{Code: l.Code, URL: l.URL, CreatedAt: l.CreatedAt, Clicks: l.Clicks}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}
