# Template Rules

Rules for building apps on top of this template.

---

## File Structure

```
build/
  main.go        — server setup, config loading, app routes           ← edit this
  auth-human.go  — /login, /logout, handleCallback, JWT, requireAuth  ← do not edit
  auth-server.go — server-to-server calls to auth-center (delegateCode) ← do not edit
  db.go          — SQLite init, users table, core queries              ← do not edit
  app_db.go      — app-specific migrations                            ← edit this
  web/
    index.html   — Go template: {{if .User}} navbar / {{else}} login  ← edit this
    style.css    — all styles (sectioned: BASE · NAVBAR · LOGIN · APP) ← edit APP section
    script.js    — all client logic (profile popover, tabs, app logic) ← edit App logic section
    favicon.svg / background.webp                                      ← swap per app
```

The `web/` folder is just three source files. Shared chrome (navbar, profile popover, login screen) and your app-specific code live together in the same files, separated by labeled sections — build inside the `APP` section of `style.css` and the `App logic` section of `script.js`. All three are embedded into the binary via `//go:embed web` and served as explicit `GET` routes in `main.go`.


**`app_db.go`** is the entry point for app-specific database work. Add your tables in `appMigrate()`. It is called automatically on startup after the core `users` table is ready.

**`main.go`** is where you add app routes and handlers. Auth routes (`/login`, `/logout`) and the request logger are already wired up — do not duplicate them.

---

## Auth Flow

```
GET /login
  → redirect to auth-center
  → auth-center authenticates user
  → redirect back to /?code=

GET /?code=...  (handleCallback in auth-human.go)
  → POST /exchange to auth-center (server-side)
  → upsertUser in DB
  → set JWT cookie
  → redirect to /
```

Use `sessionUserID(r)` anywhere to get the current user's internal DB id (returns `0` if not logged in).

Use `requireAuth(handler)` middleware to protect routes that need a logged-in user.

---

## Database

The `users` table is created automatically. It holds the minimum:

| Column | Description |
|---|---|
| `id` | Internal primary key — use this everywhere in your app |
| `auth_id` | Permanent ID from auth-center (Telegram ID, Solana pubkey, Google sub) |
| `method` | `telegram`, `solana`, or `google` |
| `name` | Display name from the auth provider |
| `created_at` | First login |
| `last_login` | Updated on every login |

Add app-specific columns and tables in `app_db.go`. Always reference `users.id` as the foreign key — never `auth_id`.

The DB file is named after the app — `<app-name>.db` — and that name is **derived, not typed twice**. `APP_NAME` is the app's identity; the database filename is a child that falls out of it:

```
APP_NAME=qcode  →  main() sets DB_PATH=qcode.db  →  db.go opens it
```

`main()` resolves `appName` and fills `DB_PATH` *before* `initDB()`, so `db.go` keeps its one-line `os.Getenv("DB_PATH")` and stays byte-identical across every app. When forking, change **`const defaultAppName` in `main.go`** and set `APP_NAME=` in the service file. **Do not edit the fallback in `db.go`** — `main()` guarantees `DB_PATH` is non-empty, so that branch is unreachable dead code kept only so the shared file is safe standalone.

An explicit `DB_PATH` always wins, which is how a compose mount pins the file to `/data`. Empty and unset are identical (`os.Getenv` returns `""` for both), so a unit file may keep `Environment=DB_PATH=` present as documentation.

⚠️ **A relative `DB_PATH` resolves against the working directory.** Always set `WorkingDirectory=` in the unit — otherwise systemd starts the process in `/` and the database lands at `/<app-name>.db`. Confirm at boot with the start line, which exists precisely so you never have to guess:

```
start app=qcode db=qcode.db port=8890
```

Changing `APP_NAME` does **not** migrate anything: the app opens a different file, SQLite creates it empty, and the app comes up with zero users looking wiped. Stop the service, `mv` the `.db` (plus any `-wal` / `-shm` sidecars), then change the variable.

New columns on an existing DB need an `ALTER TABLE` fallback:
```go
db.Exec(`ALTER TABLE users ADD COLUMN my_col TEXT`) // nolint:errcheck — ok if column exists
```

---

## Logging

Every meaningful user action must produce exactly one log line. Use `log.Printf` with a consistent `key=value` format:

```
login  uid=1 method=telegram name="Ivan" new=true
logout uid=1
```

For app-specific actions follow the same pattern:
```
action uid=1 item_id=42 result=ok
action uid=1 error="not found"
```

Rules:
- One line per action — not per code step
- Always include `uid=` when a user is involved
- Always log errors, including unexpected DB errors
- Never log secrets, tokens, or raw codes

The request logger in `main.go` covers HTTP-level logging (`GET / 200 3ms`) automatically — do not add per-route request logging on top of it.

---

## Environment Variables

| Variable | Required | Default | Description |
|---|:---:|---|---|
| `AUTH_URL` | ★ | — | Public auth-center URL (browser-facing) |
| `AUTH_INTERNAL` | ★ | — | Internal auth-center URL (server-side `/exchange`) |
| `APP_URL` | ★ | — | Public URL of this app |
| `APP_TOKEN` | ★ | — | Secret registered in auth-center's `APP_TOKENS` |
| `SECRET_KEY` | ★ | `dev-secret` | JWT signing key — always set in prod |
| `APP_NAME` | | `defaultAppName` | Runtime identity — names the DB (`<APP_NAME>.db`), the `--version` banner, and the start log line |
| `DB_PATH` | | `<APP_NAME>.db` | SQLite file path. Explicit value always wins; relative paths need `WorkingDirectory=` |
| `PORT` | | `8890` | HTTP port |

Copy `.env.example` to `.env` for local dev. Never commit `.env`.

---

## Cross-app login (delegate)

Lets a user open another app without logging in again. The receiving app needs no changes — its existing `/?code=` handling already works.

**On the sending app (App1), add a handler in `main.go`:**

```go
func handleOpenApp2(w http.ResponseWriter, r *http.Request) {
    uid := sessionUserID(r)
    if uid == 0 {
        http.Redirect(w, r, "/login", http.StatusFound)
        return
    }
    code, err := delegateCode(uid)
    if err != nil {
        log.Printf("delegate error: %v", err)
        http.Error(w, "could not open app", http.StatusInternalServerError)
        return
    }
    http.Redirect(w, r, "https://app2.example.com/?code="+code, http.StatusFound)
}
```

Register the route:
```go
mux.HandleFunc("GET /open-app2", handleOpenApp2)
```

Add a link in the UI:
```html
<a class="btn" href="/open-app2">Open App 2</a>
```

**Requirements:**
- App1 must have a valid `APP_TOKEN` in auth-center's `APP_TOKENS`
- App2 must also have a valid `APP_TOKEN` in auth-center's `APP_TOKENS`
- No other config needed — App1 does not need to know App2's token

---

## Build & Deploy

**Local dev** (hot-reload via `go run`):
```bash
docker-compose -f dev-compose.yml up client
```

**Production binary** (linux/amd64, committed to git):
```bash
docker-compose -f prod-compose.yml run --rm release
```

The binary is committed. The server does `git pull` + `systemctl restart`. No build step on the server.

Logs in production go to journalctl automatically — systemd captures stdout/stderr:
```bash
journalctl -u your-app -f
```
