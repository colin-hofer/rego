# Backend Modules

Each backend feature should live in its own package under `internal/modules/<feature>`.

## Module contract

Implement these two methods:

```go
Name() string
RegisterRoutes(mux *http.ServeMux)
```

## Where to register modules

Add your module constructor to `internal/app/modules.go`.

## Suggested package shape

```text
internal/modules/orders/
  module.go        # RegisterRoutes + handlers
  service.go       # business rules
  store_postgres.go
  model.go
```
