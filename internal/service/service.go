// Package service содержит бизнес-логику сокращения ссылок.
package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"url-shortener/internal/shortcode"
	"url-shortener/internal/storage"
)

// Ошибки сервиса.
var (
	ErrInvalidURL   = errors.New("invalid url")
	ErrInvalidAlias = errors.New("invalid alias")
	ErrAliasTaken   = errors.New("alias already taken")
	ErrNotFound     = errors.New("link not found")
)

// MaxURLLength — максимальная длина исходного URL.
const MaxURLLength = 2048

// maxGenerateAttempts — сколько раз генерировать код при коллизиях.
const maxGenerateAttempts = 5

// reserved — коды, совпадающие со служебными путями HTTP API.
var reserved = map[string]bool{"api": true, "healthz": true}

// Service — сервис коротких ссылок.
type Service struct {
	store    storage.Store
	generate func() string
	now      func() time.Time
}

// Option настраивает Service.
type Option func(*Service)

// WithGenerator подменяет генератор кодов (по умолчанию shortcode.Generate).
func WithGenerator(gen func() string) Option {
	return func(s *Service) { s.generate = gen }
}

// WithClock подменяет источник времени (по умолчанию time.Now).
func WithClock(now func() time.Time) Option {
	return func(s *Service) { s.now = now }
}

// New создаёт сервис поверх хранилища.
func New(store storage.Store, opts ...Option) *Service {
	s := &Service{store: store, generate: shortcode.Generate, now: time.Now}
	for _, o := range opts {
		o(s)
	}
	return s
}

// Shorten создаёт короткую ссылку на rawURL. Если alias непустой, он
// используется как код; иначе код генерируется.
func (s *Service) Shorten(ctx context.Context, rawURL, alias string) (storage.Link, error) {
	if err := validateURL(rawURL); err != nil {
		return storage.Link{}, err
	}
	link := storage.Link{URL: rawURL, CreatedAt: s.now().UTC()}

	if alias != "" {
		if !shortcode.ValidAlias(alias) {
			return storage.Link{}, fmt.Errorf("%w: must match [A-Za-z0-9_-]{3,32}", ErrInvalidAlias)
		}
		if reserved[alias] {
			return storage.Link{}, fmt.Errorf("%w: %q is reserved", ErrInvalidAlias, alias)
		}
		link.Code = alias
		err := s.store.Save(ctx, link)
		if errors.Is(err, storage.ErrCodeExists) {
			return storage.Link{}, ErrAliasTaken
		}
		if err != nil {
			return storage.Link{}, fmt.Errorf("save link: %w", err)
		}
		return link, nil
	}

	for attempt := 0; attempt < maxGenerateAttempts; attempt++ {
		link.Code = s.generate()
		if reserved[link.Code] {
			continue
		}
		err := s.store.Save(ctx, link)
		if errors.Is(err, storage.ErrCodeExists) {
			continue
		}
		if err != nil {
			return storage.Link{}, fmt.Errorf("save link: %w", err)
		}
		return link, nil
	}
	return storage.Link{}, fmt.Errorf("generate code: no free code after %d attempts", maxGenerateAttempts)
}

// Resolve возвращает ссылку по коду и увеличивает её счётчик переходов.
// Возвращаемая ссылка уже учитывает этот переход.
func (s *Service) Resolve(ctx context.Context, code string) (storage.Link, error) {
	link, err := s.get(ctx, code)
	if err != nil {
		return storage.Link{}, err
	}
	if err := s.store.IncrementClicks(ctx, code); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return storage.Link{}, ErrNotFound
		}
		return storage.Link{}, fmt.Errorf("increment clicks: %w", err)
	}
	link.Clicks++
	return link, nil
}

// Stats возвращает ссылку со статистикой, не меняя счётчик.
func (s *Service) Stats(ctx context.Context, code string) (storage.Link, error) {
	return s.get(ctx, code)
}

func (s *Service) get(ctx context.Context, code string) (storage.Link, error) {
	link, err := s.store.Get(ctx, code)
	if errors.Is(err, storage.ErrNotFound) {
		return storage.Link{}, ErrNotFound
	}
	if err != nil {
		return storage.Link{}, fmt.Errorf("get link: %w", err)
	}
	return link, nil
}

func validateURL(raw string) error {
	if raw == "" {
		return fmt.Errorf("%w: empty", ErrInvalidURL)
	}
	if len(raw) > MaxURLLength {
		return fmt.Errorf("%w: longer than %d bytes", ErrInvalidURL, MaxURLLength)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidURL, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("%w: scheme must be http or https", ErrInvalidURL)
	}
	if u.Host == "" {
		return fmt.Errorf("%w: empty host", ErrInvalidURL)
	}
	return nil
}
