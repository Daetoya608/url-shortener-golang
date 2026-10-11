// Package config читает настройки сервиса из переменных окружения.
package config

import (
	"os"
	"strings"
)

// Значения по умолчанию.
const (
	DefaultAddr    = ":8080"
	DefaultBaseURL = "http://localhost:8080"
	DefaultDBPath  = "shortener.db"
)

// Config — настройки сервиса.
type Config struct {
	Addr    string // ADDR: адрес HTTP-сервера
	BaseURL string // BASE_URL: префикс short_url, без завершающего "/"
	DBPath  string // DB_PATH: путь к файлу SQLite
}

// Load читает конфиг из окружения; пустые переменные заменяются значениями по умолчанию.
func Load() Config {
	baseURL := strings.TrimRight(getenv("BASE_URL", DefaultBaseURL), "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return Config{
		Addr:    getenv("ADDR", DefaultAddr),
		BaseURL: baseURL,
		DBPath:  getenv("DB_PATH", DefaultDBPath),
	}
}

func getenv(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}
