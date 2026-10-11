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
