# Журнал: url-shortener-golang

## 2026-10-12 — T1: каркас, тулчейн-скрипты, shortcode, memory-хранилище, сервис

Сделано:
- `scripts/go.sh` (go из PATH → `./.tools/go/bin/go` → скачивание go1.27.2 в `.tools/go`, через временный каталог и атомарный `mv`, curl или wget) и `scripts/test.sh` (`go vet` + `go test -count=1`). `.tools/`, `*.db` в `.gitignore`.
- `go.mod`: модуль `url-shortener`, `go 1.27`.
- `internal/shortcode`: `Generate()` — 7 символов base62 из `crypto/rand` с отбрасыванием байтов ≥ 248 (без смещения распределения); `ValidAlias`.
- `internal/storage`: `Link`, `Store`, `ErrNotFound`, `ErrCodeExists`, `Memory`; контрактные тесты `runStoreContract(t, newStore)` в `contract_test.go` — их же подключит SQLite (T-следующая).
- `internal/service`: `Shorten`/`Resolve`/`Stats`, ошибки `ErrInvalidURL`, `ErrInvalidAlias`, `ErrAliasTaken`, `ErrNotFound`.
- `cmd/shortener/main.go` — компилируемая заглушка.

Решения:
- Подмена генератора и часов — функциональные опции `WithGenerator`, `WithClock` (`service.New(store, opts...)`); пригодятся и в HTTP-тестах.
- Ошибки сервиса оборачиваются (`fmt.Errorf("%w: причина")`) — проверять через `errors.Is`; текст можно отдавать клиенту.
- Ошибки хранилища `ErrNotFound` переводятся в `service.ErrNotFound`, чтобы HTTP-слой зависел только от сервиса.
- Сгенерированный код, совпавший с зарезервированным (`healthz` — ровно 7 base62-символов), пропускается и считается попыткой.
- После 5 неудачных попыток — обычная (не sentinel) ошибка → в HTTP это будет 500.
- `Resolve` делает Get + IncrementClicks и возвращает ссылку с уже учтённым переходом (Clicks+1). Не атомарно как пара, но счётчик сам по себе атомарен — для редиректа достаточно.
- Валидация URL: сначала URL, потом alias (порядок ошибок предсказуем). Длина считается в байтах.
- `go test -race` на машине недоступен (нужен cgo/gcc); конкурентный тест счётчика есть и проходит без race-детектора.

## 2026-10-12 — T2: HTTP API, конфиг из env, точка входа

Сделано:
- `internal/config`: `Load() Config` — `ADDR`, `BASE_URL`, `DB_PATH` с умолчаниями; тесты на умолчания, переопределение и обрезку `/`.
- `internal/httpapi`: `NewRouter(svc, baseURL)` на `http.ServeMux` с шаблонами; httptest-тесты на все эндпоинты и ошибки.
- `cmd/shortener/main.go`: конфиг → memory-хранилище → сервис → роутер → `http.Server` с таймаутами, slog, graceful shutdown по SIGINT/SIGTERM (10 с). Проверено вручную: запросы curl и `kill -INT` → exit 0.
- `README.md`: запуск, env, примеры curl.

Решения:
- `Load()` без ошибки: все переменные строковые, валидировать нечего. Пустое/пробельное значение = не задано. Из `BASE_URL` срезаются все завершающие `/`; если остаётся пусто — умолчание.
- Тело > 1 МБ → `413` (а не 400), с JSON-ошибкой. Данные после JSON-объекта (`{...} {...}`) → 400. Неизвестные поля в JSON допускаются.
- Catch-all `/` отдаёт JSON 404, чтобы все ошибки были JSON. Побочный эффект: неверный метод на существующем пути (например `POST /healthz`) даёт 404 вместо 405 — приемлемо для простого API.
- Редирект с `Cache-Control: no-store`, иначе браузер закэширует ответ и клики не будут считаться (302 и так обычно не кэшируется, но явно надёжнее).
- Внутренние ошибки логируются через `slog.Default()` и отдаются клиенту как `500 {"error":"internal error"}` без деталей.
- Тексты ошибок 400/404/409 — из ошибок сервиса (`err.Error()`), они уже безопасны для клиента.
- `/healthz` — `text/plain`, тело `ok`.
- `DB_PATH` пока читается, но не используется — хранилище всегда memory до T3 (TODO в main.go).

## 2026-10-12 — T3: SQLite-хранилище и подключение в main

Сделано:
- `internal/storage/sqlite.go`: `NewSQLite(path) (*SQLiteStore, error)` на `modernc.org/sqlite` v1.60.1 (без cgo), WAL + busy_timeout 5 с, `CREATE TABLE IF NOT EXISTS links`.
- Тесты: тот же `runStoreContract` (БД в `t.TempDir()`); WAL и busy_timeout реально включены; данные (URL, created_at с наносекундами, clicks) переживают Close + повторное открытие, в т.ч. путь с пробелом; 400 конкурентных `IncrementClicks` через **два независимых пула** к одному файлу (как два процесса) — без потерь и без SQLITE_BUSY; несуществующий каталог → ошибка.
- `main`: `openStore(DB_PATH)` — `:memory:` → memory, иначе SQLite; `Close` в defer с логированием ошибки; в лог пишется тип хранилища. Тест `cmd/shortener/main_test.go` на выбор.
- `.gitignore`: добавлены `*.db-wal`, `*.db-shm` (`*.db` уже был). README и ARCHITECTURE обновлены.
- Проверено вручную: создание ссылки, переход, рестарт сервиса — ссылка и clicks=1 на месте. `CGO_ENABLED=0 go build ./...` успешна.

Решения:
- Прагмы — через DSN `_pragma=…`, а не `db.Exec` после открытия: `database/sql` держит пул, и прагма, выполненная на одном соединении, не действовала бы на остальные (busy_timeout per-connection). После открытия `journal_mode` проверяется явно — если WAL не включился (например, ФС не поддерживает), `NewSQLite` возвращает ошибку, а не тихо работает в другом режиме.
- Пул соединений не ограничен (`SetMaxOpenConns` не трогаем): в WAL читатели не блокируют писателя, а одиночные autocommit-UPDATE ждут блокировку через busy_timeout. Транзакций «чтение→запись» нет, поэтому deadlock-ситуации с SQLITE_BUSY, которую busy_timeout не лечит, не возникает.
- Дубликат кода ловится по коду ошибки драйвера (`SQLITE_CONSTRAINT_PRIMARYKEY`/`_UNIQUE`), а не через `ON CONFLICT DO NOTHING` — так другие ошибки INSERT (например NOT NULL) не маскируются под «код занят».
- Путь экранируется через `url.URL{Path}.EscapedPath()`, чтобы `?`/`#`/пробелы в `DB_PATH` не ломали DSN.
