package storage

import (
	"context"
	"sync"
)

// Memory — in-memory реализация Store (map + sync.RWMutex).
type Memory struct {
	mu    sync.RWMutex
	links map[string]Link
}

// NewMemory создаёт пустое in-memory хранилище.
func NewMemory() *Memory {
	return &Memory{links: make(map[string]Link)}
}

var _ Store = (*Memory)(nil)

func (m *Memory) Save(ctx context.Context, l Link) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.links[l.Code]; ok {
		return ErrCodeExists
	}
	m.links[l.Code] = l
	return nil
}

func (m *Memory) Get(ctx context.Context, code string) (Link, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	l, ok := m.links[code]
	if !ok {
		return Link{}, ErrNotFound
	}
	return l, nil
}

func (m *Memory) IncrementClicks(ctx context.Context, code string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	l, ok := m.links[code]
	if !ok {
		return ErrNotFound
	}
	l.Clicks++
	m.links[code] = l
	return nil
}

func (m *Memory) Close() error { return nil }
