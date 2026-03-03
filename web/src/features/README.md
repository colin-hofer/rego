# Frontend Features

Frontend code is organized by domain under `web/src/features/<name>`.

## Suggested shape

```text
web/src/features/orders/
  api.ts              # fetch wrappers for backend routes
  OrdersPanel.tsx     # feature UI
  OrdersPanel.test.tsx
```

## Registration

Render feature components from the landing page in `web/src/features/home/HomePage.tsx`.
