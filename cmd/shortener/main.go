// Command shortener — HTTP-сервис сокращения ссылок.
// Пока заглушка: полноценная сборка конфига, хранилища и HTTP-сервера — в T2.
package main

import (
	"log/slog"

	"url-shortener/internal/service"
	"url-shortener/internal/storage"
)

func main() {
	store := storage.NewMemory()
	defer store.Close()
	_ = service.New(store)
	slog.Info("shortener: stub, HTTP server is not implemented yet")
}
