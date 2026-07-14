# Auth Proxy — Endpoints

Auth-proxy is a thin pass-through — no business logic, no state, no DB. It sits
on a host that *can* reach `api.telegram.org` and bridges callers that can't
(auth-center behind a network wall; Grafana that can't reach Telegram).

## File Structure

```
build/
  main.go    — config, log middleware, main()
  proxy.go   — /webhook and /tg-api/{path...} handlers
  grafana.go — /sh-grafana handler (Grafana webhook → Telegram sendMessage)
  nomnom.go  — /nom-nom-ai handler (authenticated pass-through → Anthropic Messages API)
  github.go  — /github/{path...} handler (IP-gated reverse proxy → GitHub)
```

## Endpoints

| Method | Path | Auth | Purpose |
|---|---|---|---|
| `POST` | `/webhook` | Telegram secret header (passed through) | Forward incoming Telegram updates to auth-center |
| `POST` | `/tg-api/{path...}` | none (obscure internal path) | Forward outbound Telegram API calls to `api.telegram.org` |
| `POST` | `/sh-grafana` | `Authorization: Bearer <GRAFANA_ALERT_SECRET>` | Relay a Grafana webhook to Telegram as a chat message |
| `POST` | `/nom-nom-ai` | `Authorization: Bearer <NOM_NOM_SECRET>` | Pass a request through to the Anthropic Messages API with the real key |
| `GET`·`POST` | `/github/{path...}` | client IP in `GITHUB_ALLOWED_IPS` | Reverse-proxy to GitHub for a box that can't reach it (git pull + raw binaries) |
| `GET`  | `/` | none | Root liveness (exact match `/{$}`); deploy healthcheck probes this. Same body as `/health` |
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

### `POST /sh-grafana`
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
`https://<proxy-domain>/sh-grafana`, Authorization header `Bearer <GRAFANA_ALERT_SECRET>`.

### `GET`·`POST` `/github/{path...}`
IP-gated reverse proxy to GitHub for a box behind a network wall (Ansible/Semaphore,
deploy scripts). Client IP is read from `X-Forwarded-For` (first entry) and must be
in `GITHUB_ALLOWED_IPS`, else `403`. Two upstreams by path prefix:

- `raw/...` → `raw.githubusercontent.com/...` (binary downloads, direct)
- anything else → `github.com/...` (git smart-HTTP `git pull`, and the
  `github.com/.../raw/...` form which 302-redirects to raw — the proxy follows it)

Method, query string, headers, and body are passed through; the response is
streamed back verbatim. Registered only when `GITHUB_ALLOWED_IPS` is non-empty.
nginx must set `X-Forwarded-For` to the real client — see `tools/nginx.md`.

### `POST /nom-nom-ai`
An authenticated pass-through to the Anthropic Messages API. The proxy holds the
real `ANTHROPIC_API_KEY`; nom-nom authenticates with the shared `NOM_NOM_SECRET`
and never sees the key. Flow:

1. Compare `Authorization: Bearer <token>` against `NOM_NOM_SECRET`
   (constant-time); reject with `401` on mismatch, forwarding nothing upstream.
2. Stream the raw request body — treated as opaque — to
   `https://api.anthropic.com/v1/messages`, adding headers
   `Content-Type: application/json`, `x-api-key: <ANTHROPIC_API_KEY>`,
   `anthropic-version: 2023-06-01`.
3. Return Anthropic's response **verbatim** — same status code, same body. Errors
   are never rewritten: nom-nom inspects them (`402 Payment Required` and a body
   of `{"error":{"type":"billing_error"}}` mean "out of credits").

Notes: bodies can be ~10 MB (photo scans embed a base64 image) and are streamed,
not buffered. The upstream client has a 30s timeout — longer than nom-nom's 20s
per-call bound — so the proxy is never the first to time out on a ~15s Opus scan.
The route is registered only when both `ANTHROPIC_API_KEY` and `NOM_NOM_SECRET`
are set.

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
| `ANTHROPIC_API_KEY` | | — | Real Anthropic key; lives only on the proxy, used for `/nom-nom-ai` |
| `NOM_NOM_SECRET` | | — | Shared secret; nom-nom sends it as `Authorization: Bearer` |
| `GITHUB_ALLOWED_IPS` | | — | Comma-separated client IPs allowed to use `/github/*` (from `X-Forwarded-For`) |

`/sh-grafana` is enabled only when all three `GRAFANA_*` vars are set.
`/nom-nom-ai` is enabled only when both `ANTHROPIC_API_KEY` and `NOM_NOM_SECRET` are set.
`/github/*` is enabled only when `GITHUB_ALLOWED_IPS` is non-empty.

## Deployment

Binary committed to git. Copy to VPS-2, configure the systemd service
(`bin/auth-proxy.service`), point the Telegram webhook at
`https://<proxy-domain>/webhook`, and set
`TELEGRAM_API_URL=https://<proxy-domain>/tg-api` on auth-center.
