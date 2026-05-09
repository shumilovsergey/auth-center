# Auth Proxy — Code Rules

## File Structure

```
build/
  main.go   — config, log middleware, /health, main()
  proxy.go  — /webhook and /tg-api/{path...} handlers
```

Auth-proxy is a thin pass-through — no business logic, no state, no DB.

---

## Logging

The middleware in `main.go` covers standard HTTP logging (`POST /webhook 200 45ms`).

Handlers log errors only:

```
webhook error=upstream: connection refused
tg-api error=build_request path=botTOKEN/sendMessage: ...
```

Do not add success logs inside handlers — the middleware already covers it.

---

## Environment Variables

| Variable | Required | Default | Description |
|---|:---:|---|---|
| `AUTH_CENTER_URL` | ★ | — | Base URL of auth-center |
| `PORT` | | `8080` | HTTP port |

---

## Deployment

Binary committed to git. Copy to VPS-2, configure systemd service (`bin/example.auth-proxy.service`), point Telegram webhook at `https://<proxy-domain>/webhook`, set `TELEGRAM_API_URL=https://<proxy-domain>/tg-api` on auth-center.
