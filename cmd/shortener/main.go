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

	// TODO(T3): SQLite по cfg.DBPath; пока всегда in-memory.
	store := storage.NewMemory()
	defer store.Close()

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
		slog.Info("listening", "addr", cfg.Addr, "base_url", cfg.BaseURL, "storage", "memory")
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
