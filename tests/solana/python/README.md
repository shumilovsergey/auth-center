# Solana Auth 2.0 — Python MVP

Simple proof-of-concept for Solana wallet authentication with a Python/Flask backend.
Supports **Phantom** and **Solflare** wallets.

---

## First Start

### 1. Install a Solana wallet extension

- [Phantom](https://phantom.app) — browser extension
- [Solflare](https://solflare.com) — browser extension

### 2. Run with Docker

```bash
docker compose up --build
```

App is available at `http://localhost:8889`

> **HTTPS required in production.**
> Wallet extensions may refuse to connect on plain `http://` in production.
> For local development `http://localhost` works fine.
> When you deploy, put your app behind a reverse proxy (nginx/caddy) with a valid TLS cert.

---

## What the server returns after auth

When a user authenticates, the server returns:

| Field        | Type   | Always? | Description                          |
|--------------|--------|---------|--------------------------------------|
| `ok`         | bool   | yes     | `true` on success                    |
| `public_key` | string | yes     | **Unique Solana wallet address**      |

### What to use as a unique user identifier?

Use **`public_key`** — the Solana wallet address. It is:
- A base58-encoded 32-byte Ed25519 public key (e.g. `5ZX8wAy...`)
- Globally unique across all of Solana
- Permanent — tied to the private key which never leaves the user's device
- The same address works across all Solana apps

### How does verification work?

There is no shared secret. The flow is:
```
server issues nonce  →  wallet signs nonce with private key
→  server verifies Ed25519 signature using public key (PyNaCl)
```

The nonce is single-use and deleted after successful auth, preventing replay attacks.
This is done in `main.py` at the `/nonce` and `/auth` routes.

---

## Project structure

```
python/
├── main.py            # Flask server + Ed25519 signature verification
├── requirements.txt   # dependencies
├── Dockerfile
├── docker-compose.yml
├── .env.example
└── web/
    ├── index.html     # Hello World page + Connect Wallet button
    ├── style.css
    └── script.js      # wallet detection, sign, send to /auth
```
