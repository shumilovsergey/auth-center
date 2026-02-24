# Telegram QR Auth — Python MVP

Proof-of-concept for QR code based Telegram authentication with a Python/Flask backend.
User scans a QR code → opens bot in Telegram → taps START → authenticated.

---

## First Start

### 1. Configure environment

```bash
cp .env.example .env
```

Fill in `.env`:

```env
BOT_TOKEN="your_bot_token_here"
BOT_USERNAME="your_bot_username"   # without @
WEBHOOK_SECRET=""                  # optional, any random string
```

### 2. Expose your local server to the internet

Telegram's webhook requires a public HTTPS URL. For local dev use **ngrok**:

```bash
ngrok http 8887
# copy the https://xxxx.ngrok-free.app URL
```

> **HTTPS required.**
> Telegram only sends webhook calls to `https://` URLs.
> For production, put your app behind nginx/caddy with a valid TLS cert.

### 3. Register the webhook with Telegram

```bash
curl -X POST "https://api.telegram.org/bot<BOT_TOKEN>/setWebhook" \
  -d "url=https://your-ngrok-url.ngrok-free.app/webhook"
```

If you set a `WEBHOOK_SECRET` in `.env`, add it:

```bash
curl -X POST "https://api.telegram.org/bot<BOT_TOKEN>/setWebhook" \
  -d "url=https://your-ngrok-url.ngrok-free.app/webhook" \
  -d "secret_token=your_secret"
```

### 4. Run with Docker

```bash
docker compose up --build
```

App is available at `http://localhost:8887`

---

## What the server returns after auth

When the user scans the QR and taps START in Telegram, the frontend polling receives:

| Field        | Type    | Always? | Description                   |
|--------------|---------|---------|-------------------------------|
| `id`         | integer | yes     | **Unique Telegram user ID**   |
| `first_name` | string  | yes     | User's first name             |
| `last_name`  | string  | no      | User's last name              |
| `username`   | string  | no      | Telegram @username            |

### What to use as a unique user identifier?

Use **`id`** — the Telegram user ID. It is:
- A permanent numeric ID (e.g. `123456789`)
- Globally unique across all of Telegram
- Never changes, even if the user changes their username or phone number
- Always present in every auth response

### How does the token work?

The QR code encodes a URL: `https://t.me/your_bot?start=<token>`

The token is a cryptographically random string (43 chars, URL-safe).
It is **not** a JWT — Telegram's `?start=` parameter only allows `A-Za-z0-9_-`
(no dots), so a raw JWT cannot be used here.

Security properties:
- Single-use: deleted from memory after a successful auth
- Short-lived: expires after 5 minutes
- The server is the only place that can mark a token as authenticated (via webhook)

---

## Project structure

```
python/
├── main.py            # Flask server + QR generation + webhook handler + polling
├── requirements.txt   # dependencies
├── Dockerfile
├── docker-compose.yml
├── .env               # secrets (never commit)
├── .env.example       # template
└── web/
    ├── index.html     # Hello World page + QR code display
    ├── style.css
    └── script.js      # fetches QR, polls /poll/<token>, shows result
```
