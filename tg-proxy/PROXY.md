# tg-proxy — Telegram Webhook Proxy

## Problem

VPS-1 runs auth-center but Telegram's servers are blocked on it.  
Telegram can't reach `POST /webhook` → Telegram auth is broken.

VPS-2 has Telegram access but nothing else runs there.

## Solution

Run a minimal Go service on VPS-2. It does exactly two things:

- `POST /webhook` — forwards the request as-is to auth-center on VPS-1
- `GET /health` — returns 200 OK

Telegram's webhook is pointed at VPS-2. Auth-center stays unchanged.

```
Telegram → VPS-2 (tg-proxy :8080) → VPS-1 (auth-center :8886)
```

---

## What Gets Built

A single Go binary: `tg-proxy`

Environment variables:

| Variable | Required | Description |
|---|:---:|---|
| `AUTH_CENTER_URL` | ★ | Full base URL of auth-center, e.g. `https://auth-center.sh-development.ru` |
| `PORT` | | Port to listen on (default `8080`) |

The proxy:
- Forwards the raw request body unchanged
- Forwards `X-Telegram-Bot-Api-Secret-Token` header (auth-center validates this)
- Returns whatever status code auth-center returns
- Times out after 10s

No auth, no state, no config beyond those two vars.

---

## Deployment Roadmap

### Step 1 — Deploy tg-proxy on VPS-2

Build the binary (same Docker pattern as auth-center):

```bash
cd tg-proxy/go
docker build --target builder -t tg-proxy-builder .
docker create --name tg-proxy-extract tg-proxy-builder
docker cp tg-proxy-extract:/tg-proxy bin/tg-proxy
docker rm tg-proxy-extract
```

Copy binary to VPS-2, create systemd service:

```ini
[Service]
ExecStart=/opt/tg-proxy/tg-proxy
Environment=AUTH_CENTER_URL=https://auth-center.sh-development.ru
Environment=PORT=8080
Restart=always
```

### Step 2 — Nginx on VPS-2

Terminate SSL, proxy to tg-proxy:

```nginx
server {
    listen 443 ssl;
    server_name tg-proxy.your-domain.com;

    # ssl_certificate / ssl_certificate_key here

    location /webhook {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header X-Telegram-Bot-Api-Secret-Token $http_x_telegram_bot_api_secret_token;
    }

    location /health {
        proxy_pass http://127.0.0.1:8080;
    }
}
```

### Step 3 — Re-register Telegram webhook

Point Telegram at VPS-2 instead of auth-center directly.  
`WEBHOOK_SECRET` stays the same — auth-center still validates it, nothing changes there.

```bash
curl -X POST "https://api.telegram.org/bot<BOT_TOKEN>/setWebhook" \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://tg-proxy.your-domain.com/webhook",
    "secret_token": "<WEBHOOK_SECRET>"
  }'
```

Verify:

```bash
curl "https://api.telegram.org/bot<BOT_TOKEN>/getWebhookInfo"
```

### Step 4 — Smoke test

Send `/start` to the bot from Telegram. Auth-center should receive the webhook via the proxy and respond.

Check tg-proxy logs — you should see `200` responses forwarded back from auth-center.

---

## Changes to auth-center

**None.** Auth-center receives the same POST body and the same secret header as before. It has no idea a proxy is involved.

---

## Notes

- If auth-center is temporarily down, tg-proxy will return whatever error auth-center returns (or a 502 timeout). Telegram will retry the webhook automatically.
- The proxy does not need BOT_TOKEN — it never talks to Telegram's API directly.
- `/health` is there so you can ping it from a monitoring tool or `systemctl` watchdog to confirm the proxy is alive on VPS-2.
