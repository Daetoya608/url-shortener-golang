// Package storage описывает хранилище коротких ссылок и его реализации.
package storage

import (
	"context"
	"errors"
	"time"
)

// Ошибки хранилища.
var (
	ErrNotFound   = errors.New("storage: link not found")
	ErrCodeExists = errors.New("storage: code already exists")
)

// Link — короткая ссылка.
type Link struct {
	Code      string
	URL       string
	CreatedAt time.Time
	Clicks    int64
}

// Store — хранилище ссылок.
type Store interface {
	// Save сохраняет новую ссылку; ErrCodeExists, если код уже занят.
	Save(ctx context.Context, l Link) error
	// Get возвращает ссылку по коду; ErrNotFound, если её нет.
	Get(ctx context.Context, code string) (Link, error)
	// IncrementClicks атомарно увеличивает счётчик переходов; ErrNotFound, если ссылки нет.
	IncrementClicks(ctx context.Context, code string) error
	Close() error
}
