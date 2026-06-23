# Delegate — passing the user's name (and provider) between apps

## The problem

When app **A** delegates a logged-in user to app **B** through auth-center, app **B**
shows the user's **id** but **"no name"**.

This is **not** a bug in auth-center. auth-center is **stateless** — it stores no user
profile of its own. The `/delegate` endpoint can only forward whatever the calling app
puts in the request body. If app A sends just `user_id`, then `user_id` is all app B
gets back.

Logs from a broken delegate look like this:

```
delegate user_id=105993223363753461270 method=delegate
exchange method=delegate via=delegate user=map[id:105993223363753461270]
```

Two tells that app A sent a bare request:

- `method=delegate` → app A sent no `method`, so it defaulted to `delegate`.
- `user=map[id:...]` → only the id, because no `name` was included.

## The fix (app A — the delegating app)

App A already has the user's name: it received it from auth-center at login time, e.g.

```
exchange method=google ... user=map[email:... id:... name:Сергей Шумилов]
```

App A just needs to **store** that name (in its session/cookie/db) and **forward** it
when it calls `/delegate`.

### Before (broken)

```json
POST /delegate
{
  "user_id":   "105993223363753461270",
  "app_token": "<APP_TOKEN>"
}
```

### After (fixed)

```json
POST /delegate
{
  "user_id":   "105993223363753461270",
  "app_token": "<APP_TOKEN>",
  "method":    "google",
  "name":      "Сергей Шумилов"
}
```

## Field reference for `POST /delegate`

| Field        | Required | Notes                                                          |
|--------------|----------|----------------------------------------------------------------|
| `user_id`    | yes      | Permanent provider id (Google `sub`, Telegram id, Solana pubkey). |
| `app_token`  | yes      | Must be listed in auth-center's `APP_TOKENS`.                  |
| `method`     | no\*     | Real identity provider: `google` / `telegram` / `solana`. Defaults to `delegate` if omitted — send it so app B shows the right provider. |
| `name`       | no       | Display name (Google style). Send for Google users.           |
| `first_name` | no       | Telegram / Solana style.                                       |
| `last_name`  | no       | Telegram / Solana style.                                       |

\* Not required by the server, but you should send it.

**Send the fields that match the original provider:**

- **Google user:** `method: "google"` + `name`
- **Telegram user:** `method: "telegram"` + `first_name` (+ `last_name`)
- **Solana user:** `method: "solana"` (+ `first_name` if you have a label)

## What app B receives

auth-center echoes the fields straight through. After app B redeems the code via
`POST /exchange`:

```json
{
  "ok": true,
  "method": "google",
  "via": "delegate",
  "user": {
    "id": "105993223363753461270",
    "name": "Сергей Шумилов"
  }
}
```

`via: "delegate"` marks the transport; `method` is the true provider. App B's template
renders `name` (or `first_name`/`last_name`) and stops showing "no name".

## Summary

| Side        | Change                                                                 |
|-------------|------------------------------------------------------------------------|
| auth-center | None — already forwards `name` / `first_name` / `last_name` / `method`. |
| App A       | Persist the name from login, then include `method` + `name` (or `first_name`/`last_name`) in the `/delegate` body. |
| App B       | None — already renders the name when it's present.                     |
