# rego

Go-first starter for a Go + React app.

## Why this layout

- `net/http` from the Go standard library handles all API and static serving.
- Go owns the build pipeline (`esbuild` Go API), dev supervisor, and test runner.
- Production mode serves embedded frontend assets from the Go binary.
- Development mode supports:
  - backend rebuild + restart on Go file changes
  - incremental frontend rebuilds on web file changes (persistent esbuild context)
  - browser full-page live reload via SSE

## Commands

```bash
go run ./cmd/rego dev
go run ./cmd/rego serve
go run ./cmd/rego build
go run ./cmd/rego test
```

Useful flags:

```bash
go run ./cmd/rego dev --addr :8080
go run ./cmd/rego serve --dev --addr :8080
go run ./cmd/rego build --output bin/rego
```

## Project layout

```text
cmd/rego/main.go            # CLI entry point
internal/app                # command wiring (dev/build/serve/test)
internal/dev                # local dev watcher + process supervisor
internal/server             # net/http server, middleware, live-reload endpoints
internal/web                # frontend build orchestration + npm bootstrapping
web/src                     # React app source
web/dist                    # built frontend assets
web/embed.go                # production go:embed assets
```

## Notes

- First `dev`, `build`, or `test` run will install frontend dependencies via `npm install`.
- `web/dist` is generated automatically by `dev` and `build`; the directory stays in git with `.gitkeep` while build artifacts are ignored.
- `serve` in production mode uses embedded assets from `web/dist` at build time.
