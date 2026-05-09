# Repository Guidelines

## Project Structure & Module Organization
`cmd/evolution-go/` contains the application entrypoint. Most backend code lives in `pkg/`, organized by feature and infrastructure concerns such as `pkg/routes`, `pkg/server`, `pkg/message`, `pkg/events`, and `pkg/storage`. Generated or compiled artifacts go to `build/`. API and wiki docs live in `docs/`, Docker examples in `docker/examples/`, and static assets in `public/` plus the built manager UI in `manager/dist/`.

This module uses a local replace for `go.mau.fi/whatsmeow`, so keep `whatsmeow-lib/` present when developing locally.

## Build, Test, and Development Commands
Use the Makefile as the primary interface:

- `make setup` installs Go dependencies and generates Swagger docs.
- `make dev` runs the service in development mode.
- `make watch` runs with hot reload if `air` is installed.
- `make build` compiles `cmd/evolution-go/main.go` to `build/evolution-go`.
- `make test` runs `go test -v ./...`.
- `make check` runs formatting, vet, lint, and tests.
- `make docker-compose-up` starts the local container stack from Docker Compose.

For first-time setup, copy `.env.example` to `.env` and adjust database, API key, and storage settings.

## Coding Style & Naming Conventions
Follow standard Go conventions and let `gofmt` control formatting; use tabs as emitted by `go fmt`. Keep package names lowercase (`pkg/chatwoot`), exported symbols in `CamelCase`, and unexported helpers in `camelCase`. Prefer small, focused packages over cross-cutting utility code. Run `make fmt` and `make vet` before opening a PR; run `make lint` when `golangci-lint` is available.

## Testing Guidelines
Write tests as `*_test.go` files next to the code they cover, following the existing pattern in `pkg/utils/utils_test.go`. Prefer table-driven tests for handlers, parsers, and service logic. Run `make test` locally for every change and `make test-race` for concurrency-sensitive work. Use `make test-coverage` when touching core flows.

## Commit & Pull Request Guidelines
Recent history favors Conventional Commit-style subjects such as `feat(chatwoot): implement ...`; follow `type(scope): summary` where possible. Keep commits focused and reviewable.

PRs should match `.github/pull_request_template.md`: include a clear description, linked issue (`Closes #123`), change type, testing notes, and screenshots when UI behavior changes. Call out breaking changes, config updates, and any required follow-up work explicitly.

## Security & Configuration Tips
Do not commit real credentials or production URLs. Use `.env.example` as the baseline, and document any new environment variables in both the example file and relevant docs.
