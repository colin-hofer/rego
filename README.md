# rego

Lean starter for building Go + React + Postgres apps with minimal friction.

## What this starter optimizes for

- Go-first runtime using `net/http` (no extra backend framework).
- Single React landing page as the default UI baseline.
- Embedded frontend assets in production builds.
- Auto-managed Postgres for local development and tests.
- Fast local loop: backend restart + frontend rebuild + browser reload.

## Commands

```bash
go run ./cmd/rego dev
go run ./cmd/rego serve
go run ./cmd/rego build
go run ./cmd/rego test
go run ./cmd/rego db status
go run ./cmd/rego db shell
go run ./cmd/rego db stop
```

Useful flags:

```bash
go run ./cmd/rego dev --addr :8080
go run ./cmd/rego serve --dev --addr :8080
go run ./cmd/rego serve --database-url postgres://user:pass@localhost:5432/app?sslmode=disable
go run ./cmd/rego build --output bin/rego
go run ./cmd/rego test --database-url postgres://user:pass@localhost:5432/app?sslmode=disable
```

## Project layout

```text
cmd/rego/main.go            # CLI entry point
internal/app                # command wiring (dev/serve/db/build/test)
internal/dev                # local dev watcher + process supervisor
internal/db                 # postgres lifecycle + sql migrations
internal/server             # net/http server, API routes, middleware, live reload
internal/web                # frontend build + npm orchestration
web/src/App.tsx             # React app entry component
web/src/features/home       # single landing page
web/src/features/system     # backend health UI client
web/dist                    # built frontend assets
web/embed.go                # production go:embed assets
```

## Extending the backend

1. Add route handlers under `internal/server` (for example `route_<name>.go`).
2. Register them from `internal/server/routes_api.go`.
3. Keep domain/service logic in dedicated internal packages when complexity grows.

## Extending the frontend

1. Add UI/API code under `web/src/features/<name>/`.
2. Import and render your component from `web/src/features/home/HomePage.tsx` (or directly from `web/src/App.tsx`).

## Notes

- First `dev`, `build`, or `test` run installs frontend dependencies via `npm install`.
- `dev`, `serve`, and `test` automatically provision Postgres if `DATABASE_URL` is not set.
- Embedded Postgres binaries are downloaded once per OS/arch and reused in `.tmp/postgres`.
- SQL migrations are embedded from `internal/db/migrations` and run automatically at startup.
- `serve` in production mode serves embedded assets from `web/dist` at build time.
