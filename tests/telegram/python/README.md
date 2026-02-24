# Telegram Auth 2.0 — Python MVP

Simple proof-of-concept for Telegram Login Widget with a Python/Flask backend.

---

## First Start

### 1. Create a bot via BotFather

1. Open Telegram → `@BotFather`
2. Send `/newbot` and follow the steps
3. Copy the **bot token** you receive

### 2. Set the Web Login domain in BotFather

1. In `@BotFather` go to your bot settings → **Web Login** (or send `/setdomain`)
2. Enter your domain (see note below)
3. Save

> **HTTPS required in production.**
> Telegram's Login Widget only works on `https://` domains in production.
> For local development `http://localhost` is accepted as an exception.
> When you deploy, you must have a valid TLS certificate and set your real domain (e.g. `https://yourdomain.com`) in BotFather.

### 3. Configure environment

```bash
cp .env.example .env
```

Fill in `.env`:

```env
BOT_TOKEN="your_bot_token_here"
BOT_USERNAME="your_bot_username"   # without @
```

### 4. Run with Docker

```bash
docker compose up --build
```

App is available at `http://localhost:8888`

---

## What Telegram returns after auth

When a user authenticates, Telegram sends back this object to your frontend:

| Field        | Type    | Always? | Description                        |
|--------------|---------|---------|------------------------------------|
| `id`         | integer | yes     | **Unique Telegram user ID**        |
| `first_name` | string  | yes     | User's first name                  |
| `last_name`  | string  | no      | User's last name                   |
| `username`   | string  | no      | Telegram @username                 |
| `photo_url`  | string  | no      | Profile photo URL                  |
| `auth_date`  | integer | yes     | Unix timestamp of auth             |
| `hash`       | string  | yes     | HMAC-SHA256 signature (see below)  |

### What to use as a unique user identifier?

Use **`id`** — the Telegram user ID. It is:
- A permanent numeric ID (e.g. `123456789`)
- Globally unique across all of Telegram
- Never changes, even if the user changes their username or phone number
- Always present in every auth response

`username` is **not** reliable as a unique key — users can change or remove it.

### What is `hash`?

`hash` is not a user identifier — it is a security signature used **only for server-side verification**.

The server computes:
```
data_check_string = sorted key=value pairs joined by \n
secret_key        = SHA-256(bot_token)
expected_hash     = HMAC-SHA256(data_check_string, secret_key)
```

If `expected_hash == hash` from Telegram → the auth data is genuine and untampered.
This is done in `main.py` at the `/auth` route before trusting any user data.

---

## Project structure

```
python/
├── main.py            # Flask server + hash verification
├── requrement.txt     # dependencies
├── Dockerfile
├── docker-compose.yml
├── .env               # secrets (never commit)
├── .env.example       # template
└── web/
    ├── index.html     # Hello World page + Telegram widget
    ├── style.css
    └── script.js      # sends auth data to /auth, shows result
```
