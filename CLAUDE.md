# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

### Initial setup
```bash
# Clone the required whatsmeow fork (local replace in go.mod)
git clone git@github.com:evolution-foundation/whatsmeow.git whatsmeow-lib

make setup        # installs deps + generates Swagger docs
cp .env.example .env
make dev          # run with .env loaded (-dev flag)
```

### Development
```bash
make dev          # run in development mode (loads .env via godotenv)
make watch        # hot reload (requires: go install github.com/cosmtrek/air@latest)
make build        # compile to build/evolution-go
```

### Testing
```bash
make test                          # go test -v ./...
go test -v ./pkg/utils/...         # run a single package's tests
make test-race                     # test with race detector
make test-coverage                 # generates coverage.html
```

### Code quality
```bash
make fmt          # go fmt ./...
make vet          # go vet ./...
make lint         # golangci-lint run ./... (requires golangci-lint)
make check        # fmt + vet + lint + test
```

### Swagger
```bash
make swagger      # swag init -g cmd/evolution-go/main.go -o ./docs
# UI available at http://localhost:<SERVER_PORT>/swagger/index.html
```

### Optional tools
```bash
go install github.com/cosmtrek/air@latest
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install github.com/swaggo/swag/cmd/swag@latest
```

## Architecture

### Entry point and wiring
`cmd/evolution-go/main.go` is the single composition root. It initializes both databases, all event producers, all services, and wires them together before calling `setupRouter`. There is no dependency injection framework — everything is wired explicitly via constructors.

### Two-database design
- **Users/GORM DB** (`pkg/config/config.go → CreateUsersDB`): PostgreSQL via GORM. Stores `Instance`, `Message`, and `Label` models. Auto-migrated on startup.
- **Auth DB** (`initAuthDB` / `initPostgresAuthDB`): Either SQLite (`dbdata/users.db`) or a separate PostgreSQL DSN (`POSTGRES_AUTH_DB`). Stores raw WhatsApp session credentials (managed internally by whatsmeow). Both `sql.DB` handles are passed down to `whatsmeow.Service`.

### whatsmeow-lib
`go.mod` contains `replace go.mau.fi/whatsmeow => ./whatsmeow-lib`, pointing to a local fork. The `whatsmeow-lib/` directory must be present or the build will fail. This fork is maintained at `github.com/evolution-foundation/whatsmeow`.

### Domain package layout
Each feature domain under `pkg/` follows a consistent four-layer pattern:
```
pkg/<domain>/
  handler/    # Gin handlers — parse HTTP, call service, return JSON
  service/    # Business logic — holds *whatsmeow.Client reference
  model/      # GORM models
  repository/ # DB access (instances/messages/labels only)
```
Domains: `call`, `chat`, `community`, `group`, `instance`, `label`, `message`, `newsletter`, `poll`, `sendMessage`, `user`.

### In-memory client map
`clientPointer map[string]*whatsmeow.Client` and `killChannel map[string]chan bool` are initialized in `main.go` and passed by reference to every service. They track live WhatsApp connections keyed by instance name. The `whatsmeow.Service` (`pkg/whatsmeow/service/whatsmeow.go`) is the single place that creates, connects, and destroys `whatsmeow.Client` instances.

### Event producers
All producers implement `pkg/events/interfaces/Producer`. Four implementations: RabbitMQ (`pkg/events/rabbitmq`), NATS (`pkg/events/nats`), Webhook (`pkg/events/webhook`), WebSocket (`pkg/events/websocket`). The whatsmeow service calls them in response to incoming WhatsApp events. RabbitMQ supports reconnection even when the initial connection fails.

### License / core gating (`pkg/core/c0.go`)
All API routes return `503` until a license is activated. `GateMiddleware` checks `RuntimeContext.IsActive()` on every request. The `RuntimeContext` is initialized at startup — it tries the stored license key first, then falls back to `GLOBAL_API_KEY` by validating it against `license.evolutionfoundation.com.br`. License state persists in the `runtime_configs` PostgreSQL table. Heartbeats are sent every 30 minutes. The obfuscated variable/function names in `c0.go` are intentional — do not rename or refactor them.

### Middleware
- `AuthAdmin` (`pkg/middleware/auth_middleware.go`): checks `GLOBAL_API_KEY`; used on `/instance/create`, `/instance/all`, `/instance/delete`, etc.
- `Auth`: checks per-instance API key; used on all messaging/user/chat/group routes.
- `JIDValidationMiddleware` (`pkg/middleware/jid_validation_middleware.go`): normalizes phone numbers to WhatsApp JID format before handlers run (handles BR/MX/AR number formatting quirks).

### Manager UI
A pre-built React app served statically from `manager/dist/`. Routes `/manager/*` and `/assets/*` bypass the license gate so the registration flow is always accessible.

### JID utilities (`pkg/utils/utils.go`)
`CreateJID` and `ParseJID` normalize phone numbers to `@s.whatsapp.net` JIDs, with special-case logic for Brazilian (remove 9 prefix for DDD ≥ 31), Mexican, and Argentine numbers. This logic has thorough unit tests in `pkg/utils/utils_test.go`.
