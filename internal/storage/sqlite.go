package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// busyTimeout — сколько соединение ждёт снятия блокировки, прежде чем вернуть SQLITE_BUSY.
const busyTimeout = 5 * time.Second

const schema = `CREATE TABLE IF NOT EXISTS links (
	code       TEXT PRIMARY KEY,
	url        TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL,
	clicks     INTEGER NOT NULL DEFAULT 0
)`

// SQLiteStore — реализация Store на SQLite (modernc.org/sqlite, без cgo).
type SQLiteStore struct {
	db *sql.DB
}

var _ Store = (*SQLiteStore)(nil)

// NewSQLite открывает (или создаёт) БД по пути path, включает WAL и busy_timeout
// и создаёт таблицу links, если её нет.
func NewSQLite(path string) (*SQLiteStore, error) {
	// Прагмы в DSN применяются драйвером к каждому новому соединению пула.
	q := url.Values{}
	q.Add("_pragma", fmt.Sprintf("busy_timeout(%d)", busyTimeout.Milliseconds()))
	q.Add("_pragma", "journal_mode(WAL)")
	dsn := "file:" + (&url.URL{Path: path}).EscapedPath() + "?" + q.Encode()

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlite: open %q: %w", path, err)
	}
	s := &SQLiteStore{db: db}
	if err := s.init(); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite: init %q: %w", path, err)
	}
	return s, nil
}

func (s *SQLiteStore) init() error {
	ctx := context.Background()
	var mode string
	if err := s.db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&mode); err != nil {
		return err
	}
	if !strings.EqualFold(mode, "wal") {
		return fmt.Errorf("journal_mode = %q, want wal", mode)
	}
	_, err := s.db.ExecContext(ctx, schema)
	return err
}

func (s *SQLiteStore) Save(ctx context.Context, l Link) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO links (code, url, created_at, clicks) VALUES (?, ?, ?, ?)`,
		l.Code, l.URL, l.CreatedAt.UTC(), l.Clicks)
	if err != nil {
		var se *sqlite.Error
		if errors.As(err, &se) && (se.Code() == sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY || se.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE) {
			return ErrCodeExists
		}
		return fmt.Errorf("sqlite: save %q: %w", l.Code, err)
	}
	return nil
}

func (s *SQLiteStore) Get(ctx context.Context, code string) (Link, error) {
	var l Link
	err := s.db.QueryRowContext(ctx,
		`SELECT code, url, created_at, clicks FROM links WHERE code = ?`, code).
		Scan(&l.Code, &l.URL, &l.CreatedAt, &l.Clicks)
	if errors.Is(err, sql.ErrNoRows) {
		return Link{}, ErrNotFound
	}
	if err != nil {
		return Link{}, fmt.Errorf("sqlite: get %q: %w", code, err)
	}
	return l, nil
}

func (s *SQLiteStore) IncrementClicks(ctx context.Context, code string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE links SET clicks = clicks + 1 WHERE code = ?`, code)
	if err != nil {
		return fmt.Errorf("sqlite: increment %q: %w", code, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("sqlite: increment %q: %w", code, err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLiteStore) Close() error { return s.db.Close() }
