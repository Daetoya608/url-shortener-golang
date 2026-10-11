# Архитектура — url-shortener-golang

Простой сервис сокращения ссылок: HTTP API принимает длинный URL, выдаёт короткий код, по коду делает редирект и считает переходы.

## Стек

| Что | Выбор | Почему |
|---|---|---|
| Язык | Go 1.27 (`go 1.27` в `go.mod`) | Требование пользователя |
| HTTP | стандартный `net/http` с роутингом по шаблонам (`mux.HandleFunc("GET /{code}", …)`) | Фреймворк не нужен, роутинг stdlib с Go 1.22 умеет методы и параметры пути |
| Хранилище | SQLite через `modernc.org/sqlite` (pure Go, без cgo) + in-memory реализация для тестов | Данные переживают рестарт, нет внешнего сервиса; gcc на машине нет, поэтому без cgo |
| Логи | `log/slog` | stdlib |
| Тесты | `testing` + `net/http/httptest` | stdlib, без сторонних assert-библиотек |

## Окружение и запуск тестов

На машине **нет** Go в `PATH` и нет доступа к Docker. Поэтому тесты запускаются через `scripts/test.sh`:

1. если `go` есть в `PATH` — использует его;
2. иначе использует `./.tools/go/bin/go`, а если его нет — скачивает `https://go.dev/dl/go1.27.2.linux-amd64.tar.gz` и распаковывает в `./.tools/go`;
3. выполняет `go vet ./...` и `go test ./... -count=1`.

`.tools/` добавлен в `.gitignore`. Кэши Go по умолчанию (`~/go`, `~/.cache/go-build`) — допустимо.

```bash
./scripts/test.sh          # команда тестов (test_command)
./scripts/go.sh run ./cmd/shortener   # запуск сервиса тем же тулчейном
```

`scripts/go.sh` — тонкая обёртка: та же логика поиска/установки Go, затем `exec go "$@"`. `test.sh` использует её.

## Структура

```
cmd/shortener/main.go        — точка входа: конфиг, хранилище, HTTP-сервер, graceful shutdown
internal/config/             — конфиг из переменных окружения
internal/shortcode/          — генерация кодов (crypto/rand, base62, длина 7)
internal/storage/            — интерфейс Store, ошибки, реализации:
    memory.go                — map + sync.RWMutex (тесты, режим без БД)
    sqlite.go                — SQLite (modernc.org/sqlite), схема создаётся при старте
internal/service/            — бизнес-логика: валидация URL, генерация кода с ретраями при коллизии
internal/httpapi/            — HTTP-хендлеры, JSON, роутинг
scripts/                     — go.sh, test.sh
```

## Ключевые решения

- **Интерфейс хранилища** (`internal/storage`):
  ```go
  type Link struct { Code, URL string; CreatedAt time.Time; Clicks int64 }
  type Store interface {
      Save(ctx context.Context, l Link) error            // ErrCodeExists при дубликате кода
      Get(ctx context.Context, code string) (Link, error) // ErrNotFound
      IncrementClicks(ctx context.Context, code string) error
      Close() error
  }
  ```
  Обе реализации проходят один и тот же набор контрактных тестов (общая функция `runStoreContract(t, newStore)`).
- **Коды**: 7 символов base62 из `crypto/rand`. При `ErrCodeExists` сервис повторяет генерацию до 5 раз. Пользователь может передать свой `alias` (`^[A-Za-z0-9_-]{3,32}$`); занятый alias → 409.
- **Валидация URL**: `url.Parse`, схема только `http`/`https`, непустой host, длина ≤ 2048.
- **SQLite** (`storage.NewSQLite(path)`): DSN `file:<path>?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)` — прагмы применяются к каждому соединению пула `database/sql`; после открытия проверяется, что `journal_mode` действительно `wal`. `created_at` пишется в UTC, колонка `TIMESTAMP` — драйвер читает её обратно в `time.Time`. Дубликат кода определяется по коду ошибки `SQLITE_CONSTRAINT_PRIMARYKEY` → `ErrCodeExists`; `IncrementClicks` — один `UPDATE … clicks = clicks + 1`, `RowsAffected()==0` → `ErrNotFound`.
- **Выбор хранилища** в `main`: `DB_PATH == ":memory:"` → `storage.NewMemory()`, иначе SQLite; `Close` при завершении.
- **Дубли длинных URL не дедуплицируются** — каждый POST создаёт новый код (проще и предсказуемее).
- **Зарезервированные пути**: `api`, `healthz` нельзя использовать как alias.

## HTTP API

| Метод и путь | Описание | Ответы |
|---|---|---|
| `POST /api/shorten` | тело `{"url": "...", "alias": "опц."}` | 201 `{"code","short_url","url"}`; 400 невалидный JSON/URL/alias; 409 alias занят |
| `GET /{code}` | редирект на исходный URL, +1 к счётчику | 302 с `Location`; 404 |
| `GET /api/links/{code}` | статистика | 200 `{"code","url","created_at","clicks"}`; 404 |
| `GET /healthz` | проверка живости | 200 `ok` |

Ошибки — JSON `{"error": "сообщение"}`. `short_url` = `BASE_URL + "/" + code`.

## Конфигурация (env)

| Переменная | По умолчанию | Описание |
|---|---|---|
| `ADDR` | `:8080` | адрес HTTP-сервера |
| `BASE_URL` | `http://localhost:8080` | префикс для `short_url` |
| `DB_PATH` | `shortener.db` | путь к файлу SQLite; значение `:memory:` → in-memory хранилище |

## Тестирование

- Юнит-тесты на `shortcode`, `service` (с memory-хранилищем), `config`.
- Контрактные тесты `Store` для memory и SQLite (SQLite — во временном файле `t.TempDir()`).
- HTTP-тесты через `httptest` на собранном роутере (`httpapi.NewRouter(svc, baseURL)`).
- Тесты не требуют сети и внешних сервисов (кроме первичной загрузки тулчейна и модулей).
