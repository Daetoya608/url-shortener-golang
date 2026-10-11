# План: url-shortener-golang

> Сгенерировано Dayforge. Не редактировать вручную.
> Версия плана: 1 · Прогноз окончания: 12.10.2026 · День 1 из 1

**Простой URL shortener на Go 1.27: HTTP API на stdlib net/http (создание короткой ссылки, опциональный alias, редирект, статистика переходов), хранилище SQLite (modernc.org/sqlite, без cgo) с in-memory реализацией для тестов.**

Тесты: `./scripts/test.sh`

## Вехи

- [ ] M1 — Рабочий URL shortener: API, редирект, статистика, хранение в SQLite (v0.1.0)

## День 1 — Работающий сервис сокращения ссылок с хранением в SQLite и зелёными тестами ⏳

- [x] **T1** Каркас проекта, тулчейн-скрипты, генерация кодов, in-memory хранилище и сервис
- [ ] **T2** HTTP API, конфиг из env и точка входа с graceful shutdown

  Реализовать internal/httpapi с конструктором NewRouter(svc *service.Service, baseURL string) http.Handler на stdlib ServeMux с шаблонами маршрутов: POST /api/shorten (JSON {"url","alias"?} → 201 {"code","short_url","url"}; 400 на невалидный JSON/URL/alias; 409 если alias занят), GET /{code} (302 на исходный URL с заголовком Location, счётчик +1; 404), GET /api/links/{code} (200 {"code","url","created_at","clicks"}; 404), GET /healthz (200 "ok"). Ошибки — JSON {"error": "..."}, Content-Type application/json. Ограничить тело запроса (http.MaxBytesReader, 1 МБ). internal/config: Load() читает ADDR (по умолчанию :8080), BASE_URL (http://localhost:8080, без завершающего /), DB_PATH (shortener.db). cmd/shortener/main.go: загрузка конфига, создание хранилища (пока всегда memory — SQLite подключится в T3), сервиса и роутера, http.Server с таймаутами, логирование через slog, graceful shutdown по SIGINT/SIGTERM. Добавить README.md с примерами curl.

  Критерии:
  - httptest-тесты покрывают все эндпоинты: успешное создание (201, short_url = BASE_URL/код), создание с alias, 400 на невалидный JSON, 400 на невалидный URL, 409 на занятый alias, 302 редирект с корректным Location, 404 на неизвестный код, статистика показывает clicks после редиректа, /healthz → 200
  - Ответы с ошибками — JSON с полем error
  - Тесты config: значения по умолчанию и переопределение через env (t.Setenv)
  - ./scripts/go.sh build ./... успешно; сервис стартует через ./scripts/go.sh run ./cmd/shortener и корректно завершается по Ctrl+C
  - README.md описывает запуск, переменные окружения и примеры curl
  - ./scripts/test.sh зелёный

- [ ] **T3** SQLite-хранилище и подключение его в main

  Добавить internal/storage/sqlite.go на modernc.org/sqlite (pure Go, без cgo — на машине нет gcc): NewSQLite(path string) (*SQLiteStore, error) открывает БД, включает WAL (PRAGMA journal_mode=WAL) и busy_timeout, создаёт таблицу links(code TEXT PRIMARY KEY, url TEXT NOT NULL, created_at TIMESTAMP NOT NULL, clicks INTEGER NOT NULL DEFAULT 0) если её нет. Нарушение уникальности кода маппить в ErrCodeExists, отсутствие строки — в ErrNotFound, IncrementClicks — атомарный UPDATE clicks = clicks + 1 (несуществующий код → ErrNotFound). Прогнать на SQLite тот же runStoreContract, что и для memory (БД в t.TempDir()). В main: если DB_PATH == ":memory:" — memory-хранилище, иначе SQLite; Close при завершении. Обновить README и при необходимости docs/ARCHITECTURE.md.

  Критерии:
  - Контрактные тесты Store проходят и для memory, и для SQLite
  - Тест: данные сохраняются после закрытия и повторного открытия той же БД (ссылка и счётчик clicks на месте)
  - Конкурентные IncrementClicks на SQLite не теряют обновлений и не падают с SQLITE_BUSY
  - main выбирает хранилище по DB_PATH; по умолчанию создаётся файл shortener.db, который игнорируется git
  - Сборка без cgo: CGO_ENABLED=0 ./scripts/go.sh build ./... успешна
  - ./scripts/test.sh зелёный
