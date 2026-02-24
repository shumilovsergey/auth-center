# Auth Center

A standalone authentication service. Handles how users prove who they are — your app never deals with Telegram or wallets directly.

---

## Concept

Auth Center is essentially a lightweight OAuth provider — but instead of "Login with Google", it offers login via Telegram and Solana wallet.

The idea is the same: **your app trusts Auth Center to identify the user, and Auth Center trusts the external identity providers (Telegram, Solana).**

```
User ←→ Auth Center ←→ Telegram
                   ←→ Solana
User ←→ App ←→ Auth Center
```

Auth Center sits between your app and the complexity of external auth methods. Your app only ever talks to Auth Center — it doesn't know or care whether the user authenticated via QR code, wallet signature, or anything else.

---

## User flow

```
1. User opens app (mydomain.com/app)
2. App has no session → redirects to mydomain.com/auth?redirect=mydomain.com/app
3. User authenticates on Auth Center (Telegram QR, Solana wallet, etc.)
4. Auth Center generates a short-lived one-time code
5. Auth Center redirects user back → mydomain.com/app?code=<one-time-code>
6. App backend calls Auth Center internally: POST /exchange { code }
7. Auth Center validates the code, returns user info, deletes the code
8. App creates its own session (cookie) and logs the user in
```

The user experiences it as a smooth redirect — authenticate → instantly back in the app.

---

## The one-time code

The `?code=` in the redirect URL is not a session token. It is a single-use, short-lived (30 seconds) secret that only the app backend can exchange for user info.

This means:
- Even if someone intercepts the redirect URL, the code is already burned
- The browser never holds anything sensitive long-term
- The app independently verifies every login by calling Auth Center directly

---

## What each service is responsible for

| | Auth Center | Your App |
|---|---|---|
| Knows about | Telegram, Solana, wallets | Business logic, content |
| Stores | One-time codes (30s TTL) | User sessions (days/weeks) |
| Issues | Identity confirmation | Session cookies |
| Unique user ID | Produces it | Stores and uses it |

Auth Center answers one question: **who is this person?**
Your app answers a different question: **what can this person do?**

---

## Internal communication

In production both services run in the same Docker network. When the app exchanges a code, it calls Auth Center directly over the internal network — the request never touches the public internet.

```
app → http://auth:5000/exchange   (internal, private)
user → mydomain.com/auth          (public, HTTPS)
user → mydomain.com/app           (public, HTTPS)
```

---

## Unique user identifier

Every auth method produces a permanent, unique identifier:

| Method | Unique ID |
|---|---|
| Telegram | Numeric user ID (e.g. `123456789`) |
| Solana | Wallet public key (e.g. `5ZX8w...`) |

Auth Center returns this ID to your app. Your app uses it as the primary key for that user in its own database — regardless of which method they used to log in.

