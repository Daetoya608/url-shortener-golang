# url-shortener

Сервис сокращения ссылок на Go: принимает длинный URL, выдаёт короткий код, по коду делает редирект и считает переходы.

## Запуск

Go в `PATH` не обязателен: `scripts/go.sh` при первом запуске скачает тулчейн в `./.tools/go`.

```bash
./scripts/go.sh run ./cmd/shortener     # запуск сервиса (Ctrl+C — корректная остановка)
./scripts/go.sh build ./...             # сборка
./scripts/test.sh                       # go vet + тесты
```

Сервис завершается по SIGINT/SIGTERM: перестаёт принимать соединения и дожидается текущих запросов (до 10 с).

## Переменные окружения

| Переменная | По умолчанию | Описание |
|---|---|---|
| `ADDR` | `:8080` | адрес HTTP-сервера |
| `BASE_URL` | `http://localhost:8080` | префикс для `short_url` (завершающий `/` отбрасывается) |
| `DB_PATH` | `shortener.db` | путь к файлу SQLite (создаётся при старте); `:memory:` — in-memory хранилище, данные теряются при остановке |

```bash
ADDR=:9000 BASE_URL=https://sho.rt ./scripts/go.sh run ./cmd/shortener
DB_PATH=:memory: ./scripts/go.sh run ./cmd/shortener   # без файла БД
```

## Хранилище

По умолчанию ссылки и счётчики переходов хранятся в SQLite-файле `shortener.db` (драйвер `modernc.org/sqlite` — чистый Go, cgo и gcc не нужны: `CGO_ENABLED=0 ./scripts/go.sh build ./...`). БД работает в режиме WAL, рядом с файлом появляются `shortener.db-wal` и `shortener.db-shm`; все три игнорируются git. Таблица создаётся автоматически при первом запуске.

## API

Ошибки возвращаются как JSON `{"error": "сообщение"}`. Тело запроса — не больше 1 МБ.

### Создать короткую ссылку — `POST /api/shorten`

```bash
curl -s -X POST localhost:8080/api/shorten \
  -H 'Content-Type: application/json' \
  -d '{"url": "https://go.dev/doc/"}'
# 201 {"code":"aZ3kQ9x","short_url":"http://localhost:8080/aZ3kQ9x","url":"https://go.dev/doc/"}
```

Со своим кодом (`alias`: `[A-Za-z0-9_-]{3,32}`, кроме `api` и `healthz`):

```bash
curl -s -X POST localhost:8080/api/shorten \
  -d '{"url": "https://go.dev/doc/", "alias": "go-docs"}'
# 201 {"code":"go-docs","short_url":"http://localhost:8080/go-docs","url":"https://go.dev/doc/"}
```

Ответы: `201` — создано; `400` — невалидный JSON, URL (только `http`/`https`, ≤ 2048 байт) или alias; `409` — alias занят; `413` — тело больше 1 МБ.

### Перейти по ссылке — `GET /{code}`

```bash
curl -i localhost:8080/go-docs
# HTTP/1.1 302 Found
# Location: https://go.dev/doc/
```

Каждый переход увеличивает счётчик. Неизвестный код — `404`.

### Статистика — `GET /api/links/{code}`

```bash
curl -s localhost:8080/api/links/go-docs
# 200 {"code":"go-docs","url":"https://go.dev/doc/","created_at":"2026-10-12T12:00:00Z","clicks":1}
```

### Проверка живости — `GET /healthz`

```bash
curl -s localhost:8080/healthz
# ok
```
