// Command shortener — HTTP-сервис сокращения ссылок.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"url-shortener/internal/config"
	"url-shortener/internal/httpapi"
	"url-shortener/internal/service"
	"url-shortener/internal/storage"
)

const shutdownTimeout = 10 * time.Second

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	if err := run(); err != nil {
		slog.Error("shortener stopped with error", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := config.Load()

	store, kind, err := openStore(cfg.DBPath)
	if err != nil {
		return err
	}
	defer func() {
		if err := store.Close(); err != nil {
			slog.Error("close storage", "err", err)
		}
	}()

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           httpapi.NewRouter(service.New(store), cfg.BaseURL),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", cfg.Addr, "base_url", cfg.BaseURL, "storage", kind)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}
	stop() // повторный Ctrl+C завершит процесс сразу

	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	if err := <-errCh; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	slog.Info("stopped")
	return nil
}

// openStore выбирает хранилище по DB_PATH: ":memory:" — in-memory, иначе файл SQLite.
func openStore(dbPath string) (storage.Store, string, error) {
	if dbPath == ":memory:" {
		return storage.NewMemory(), "memory", nil
	}
	s, err := storage.NewSQLite(dbPath)
	if err != nil {
		return nil, "", err
	}
	return s, "sqlite:" + dbPath, nil
}
