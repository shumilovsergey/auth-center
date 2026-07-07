# Auth Proxy — Endpoints

Auth-proxy is a thin pass-through — no business logic, no state, no DB. It sits
on a host that *can* reach `api.telegram.org` and bridges callers that can't
(auth-center behind a network wall; Grafana that can't reach Telegram).

## File Structure

```
build/
  main.go    — config, log middleware, main()
  proxy.go   — /webhook and /tg-api/{path...} handlers
  grafana.go — /alert handler (Grafana webhook → Telegram sendMessage)
```

## Endpoints

| Method | Path | Auth | Purpose |
|---|---|---|---|
| `POST` | `/webhook` | Telegram secret header (passed through) | Forward incoming Telegram updates to auth-center |
| `POST` | `/tg-api/{path...}` | none (obscure internal path) | Forward outbound Telegram API calls to `api.telegram.org` |
| `POST` | `/alert` | `Authorization: Bearer <GRAFANA_ALERT_SECRET>` | Relay a Grafana webhook to Telegram as a chat message |
| `GET`  | `/health` | none | Liveness; returns `{"status":"ok","upstream":<AUTH_CENTER_URL>}` |

### `POST /webhook`
Rebuilds the request against `AUTH_CENTER_URL/webhook`, preserving `Content-Type`
and the `X-Telegram-Bot-Api-Secret-Token` header. Point the Telegram webhook at
`https://<proxy-domain>/webhook`.

### `POST /tg-api/{path...}`
Passes the path through as-is:
`/tg-api/botTOKEN/sendMessage → api.telegram.org/botTOKEN/sendMessage`.
Set `TELEGRAM_API_URL=https://<proxy-domain>/tg-api` on auth-center so its
outbound Telegram calls route through here.

### `POST /alert`
The one endpoint that holds Telegram creds itself. Flow:

1. Compare `Authorization: Bearer <token>` against `GRAFANA_ALERT_SECRET`
   (constant-time); reject with `401` on mismatch.
2. Read the body (capped at 1 MB).
3. Send it to Telegram via `sendMessage` using the proxy's own
   `GRAFANA_BOT_TOKEN` / `GRAFANA_CHAT_ID` — **Grafana never sees the bot token.**

Currently the raw request body is forwarded verbatim as the message text
(truncated to 4000 chars, Telegram's cap is 4096) so the real Grafana payload
shape is visible before a formatter is built. The route is registered only when
all three `GRAFANA_*` vars are set.

Grafana setup: create a **Webhook** contact point → URL
`https://<proxy-domain>/alert`, Authorization header `Bearer <GRAFANA_ALERT_SECRET>`.

## Logging

The middleware in `main.go` covers standard HTTP logging (`POST /webhook 200 45ms`).
Handlers log errors only — never success (the middleware already covers it):

```
webhook error=upstream: connection refused
tg-api error=build_request path=botTOKEN/sendMessage: ...
alert error=unauthorized
alert error=telegram: telegram status=400 body=...
```

## Environment Variables

| Variable | Required | Default | Description |
|---|:---:|---|---|
| `AUTH_CENTER_URL` | ★ | — | Base URL of auth-center |
| `PORT` | | `8080` | HTTP port |
| `GRAFANA_BOT_TOKEN` | | — | Telegram bot token used to send alerts |
| `GRAFANA_CHAT_ID` | | — | Telegram chat the alert message goes to |
| `GRAFANA_ALERT_SECRET` | | — | Shared secret; Grafana sends it as `Authorization: Bearer` |

`/alert` is enabled only when all three `GRAFANA_*` vars are set.

## Deployment

Binary committed to git. Copy to VPS-2, configure the systemd service
(`bin/auth-proxy.service`), point the Telegram webhook at
`https://<proxy-domain>/webhook`, and set
`TELEGRAM_API_URL=https://<proxy-domain>/tg-api` on auth-center.
