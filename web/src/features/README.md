# Frontend Features

Each feature should live in `web/src/features/<feature>`.

## Suggested shape

```text
web/src/features/orders/
  api.ts            # fetch wrappers for backend routes
  OrdersPanel.tsx   # feature UI
  OrdersPanel.test.tsx
```

## Where to register feature routes

Add the feature page component in `web/src/app/routes.tsx`.
