# User Menu — reusable spec

A self-contained spec for the avatar → dropdown "user menu" used in the
sh-development app platform, so it can be rebuilt 1:1 in another app. Focus is on
**structure, the buttons, and what they do** — not exact colors. The one color rule
worth keeping: **log out is red.**

This matches the live implementation in `auth-client` (server-rendered Go template +
shared `shell.css` / `shell.js`). If your app renders HTML on the server, copy it as-is.
If your app is a JS SPA, see "Client-rendered variant" at the end.

## What it is

A circular avatar button in the top nav. Clicking it toggles a popover anchored to the
button. The popover has two parts:

1. **Identity block** (read-only) — provider label, display name, permanent user id.
2. **Action list** — a modern grouped list (one rounded box, rows divided by hairlines,
   no per-row borders) with: **apps**, **readme**, **log out**.

```
┌──────────────────────────────┐
│ GOOGLE                        │  ← provider (uppercase, muted)
│ Сергей Шумилов                │  ← display name
│ 105993223363753461270         │  ← permanent user id (auth provider id)
│ ┌──────────────────────────┐  │
│ │ apps                     │  │  ← delegates to the menu app (see below)
│ ├──────────────────────────┤  │  ← hairline divider between rows
│ │ readme                   │  │  ← external link, new tab
│ ├──────────────────────────┤  │
│ │ log out                  │  │  ← RED text/hover, clears session
│ └──────────────────────────┘  │
└──────────────────────────────┘
```

> There is intentionally **no** "member since" / "last login" row — the menu stays to
> identity + actions only.

## Structure (HTML)

Server-rendered. `{{...}}` are Go `html/template` fields from the logged-in `User`
(`Method`, `Name`, `AuthID`); swap them for your own templating. The avatar shows the
first letter of the name (filled in by JS from `data-name`).

```html
<div class="profile-area">
  <!-- avatar trigger: JS sets the initial from data-name -->
  <button class="profile-btn" id="profileBtn"
          data-name="{{if .User.Name}}{{.User.Name}}{{else}}{{.User.AuthID}}{{end}}">
    <span class="profile-initial">?</span>
  </button>

  <!-- popover -->
  <div class="profile-popover" id="profilePopover">
    <!-- identity block -->
    <div class="pi-info">
      <span class="pi-label">{{.User.Method}}</span>                                  <!-- e.g. google -->
      <span class="pi-name">{{if .User.Name}}{{.User.Name}}{{else}}{{.User.AuthID}}{{end}}</span>
      {{if .User.Name}}<span class="pi-sub">{{.User.AuthID}}</span>{{end}}            <!-- permanent id -->
    </div>

    <!-- grouped action list -->
    <div class="pi-list">
      <a class="pi-action" href="/apps">apps</a>
      <a class="pi-action" href="https://github.com/<you>/<repo>/blob/main/README.md"
         target="_blank" rel="noopener">readme</a>
      <a class="pi-action pi-logout" href="/logout">log out</a>
    </div>
  </div>
</div>
```

Key points:
- Trigger and popover live in the same positioned wrapper (`.profile-area`,
  `position: relative`) so the popover can anchor to the button.
- The rows are wrapped in a single `.pi-list` container — that wrapper is what creates
  the grouped-list look (one rounded box, hairline dividers).
- Action rows can be `<a>` (navigation/link) or `<button>` (JS action) — they share the
  `.pi-action` class so they look identical.
- `pi-logout` is the only row with its own color treatment (red).

## The buttons — what each one does

| Button   | Element              | Action |
|----------|----------------------|--------|
| **apps** | `<a href="/apps">` | Full-page navigation to the app's own `/apps` endpoint. The server delegates the logged-in identity to the central **menu app** and redirects there (see "Delegation" below). |
| **readme** | `<a target="_blank">` | Opens the project README in a new tab. Pure link, no JS. |
| **log out** | `<a href="/logout">` | Hits the app's `/logout` route (clears the session cookie), which redirects back to the login screen. **Red** to mark it as the destructive action. |

> The reference app logs out with a plain `GET /logout` link because that's what the
> shared auth layer exposes. If you'd rather avoid GET side-effects, make it a
> `<button>` that does `POST /logout` then reloads — the row styling is identical.

## Behavior (JS)

Lives in the shared `shell.js`. It (1) fills the avatar initial from `data-name`, and
(2) toggles the popover open/closed, closing on an outside click.

```js
const profileBtn = document.getElementById('profileBtn');
if (profileBtn) {
  // avatar initial = first letter of the name
  const name    = profileBtn.dataset.name || '';
  const initial = profileBtn.querySelector('.profile-initial');
  if (initial) initial.textContent = (name[0] || '?').toUpperCase();

  // toggle the popover; keep the trigger's .open style in sync
  const popover = document.getElementById('profilePopover');
  profileBtn.addEventListener('click', () => {
    const open = popover.classList.toggle('open');
    profileBtn.classList.toggle('open', open);
  });

  // click outside the menu → close it
  document.addEventListener('click', e => {
    if (!profileBtn.contains(e.target) && !popover.contains(e.target)) {
      popover.classList.remove('open');
      profileBtn.classList.remove('open');
    }
  });
}
```

Open/close is driven entirely by an `.open` class toggled on both the popover and the
trigger. `display: none` by default; `.open` switches it to `display: flex`.

## Styling (the actual CSS)

The recipe that makes it look "modern list" rather than "stack of buttons": a single
rounded `.pi-list` box with `overflow: hidden`, rows with no border of their own, and a
top border on every row **except the first**.

```css
.profile-area { position: relative; }

/* avatar trigger */
.profile-btn {
  width: 54px; height: 54px; border-radius: 50%;
  background: rgba(255, 255, 255, 0.12);
  border: 1px solid rgba(255, 255, 255, 0.50);
  color: rgba(255, 255, 255, 0.88);
  font-size: 18px; font-weight: 600;
  display: flex; align-items: center; justify-content: center;
  cursor: pointer;
  backdrop-filter: blur(24px) saturate(1.6);          /* glass look */
  -webkit-backdrop-filter: blur(24px) saturate(1.6);
  text-shadow: 0 1px 4px rgba(0,0,0,0.8), 0 2px 12px rgba(0,0,0,0.4);
  transition: border-color .15s, color .15s;
}
.profile-btn:hover,
.profile-btn.open { border-color: rgba(255,255,255,0.80); color: #fff; }

/* popover: floating card anchored under the trigger */
.profile-popover {
  position: absolute; top: calc(100% + 10px); right: 0;
  width: 256px; border-radius: 16px; padding: 14px; z-index: 101;
  background: rgba(0, 0, 0, 0.42);                     /* translucent dark */
  border: 1px solid rgba(255, 255, 255, 0.12);
  display: none; flex-direction: column; gap: 8px;
  backdrop-filter: blur(24px) saturate(1.6);          /* glass blur, kept flat (no shadow) */
  -webkit-backdrop-filter: blur(24px) saturate(1.6);
}
.profile-popover.open { display: flex; }

/* identity block */
.pi-info  { display: flex; flex-direction: column; gap: 3px; }
.pi-label { font-size: 10px; color: rgba(255,255,255,0.45); letter-spacing: .1em; text-transform: uppercase; }
.pi-name  { color: rgba(255,255,255,0.92); word-break: break-all; }
.pi-sub   { font-size: 11px; color: rgba(255,255,255,0.45); word-break: break-all; }

/* grouped action list: ONE rounded box, rows divided by hairlines */
.pi-list {
  display: flex; flex-direction: column;
  border: 1px solid rgba(255,255,255,0.12);
  border-radius: 12px; overflow: hidden;              /* clip rows to the rounded corners */
}
.pi-action {
  display: block; width: 100%; text-align: left;
  padding: 11px 14px; border: none; border-radius: 0;
  background: transparent; color: rgba(255,255,255,0.88);
  text-decoration: none; font: inherit; font-size: 13px;
  cursor: pointer; transition: background .15s, color .15s;
}
/* divider only BETWEEN rows, never on the first one */
.pi-action + .pi-action { border-top: 1px solid rgba(255,255,255,0.10); }
.pi-action:hover { background: rgba(255,255,255,0.06); color: rgba(255,255,255,0.95); }

/* the one intentional color: log out is red */
.pi-logout       { color: #ff9f9f; }
.pi-logout:hover { color: #ffbfbf; background: rgba(255,80,80,0.12); }
```

The grouped-list recipe in three rules:
1. Wrap rows in a container with a border + `border-radius` + `overflow: hidden`.
2. Give each row no border of its own, only padding.
3. Add a top border to every row **except the first** (`.pi-action + .pi-action`).

## Delegation (the "apps" button)

"apps" hands the logged-in user to the central **menu app** through a stateless
auth-center, passing the **name and provider** so the menu app shows the real name, not
"no name". (Same mechanism works for handing off to any other app — just change the
redirect target.)

### Server endpoint (`GET /apps`, behind auth)

1. Load the current user's permanent id, display name, and provider from your DB.
2. `POST {AUTH_INTERNAL}/delegate` with:
   ```json
   {
     "user_id":    "<permanent provider id>",
     "app_token":  "<this app's APP_TOKEN>",
     "method":     "google",            // the REAL provider, so the menu app shows it
     "name":       "Сергей Шумилов",    // Google-style display name
     "first_name": "Сергей Шумилов"     // Telegram/Solana-style fallback
   }
   ```
3. Read the returned one-time `code`.
4. `302` redirect the browser to `https://menu.sh-development.ru/?code=<code>`.

The menu app then redeems the code via its own `POST /exchange` and renders the name.

### Why name + method must be forwarded

auth-center is **stateless** — it only forwards what the caller sends. If your app sends
just `user_id`, the receiver gets only the id and shows "no name". So persist the name
you got at login and forward `method` + `name` (or `first_name`/`last_name`) on every
`/delegate` call.

| Field        | Required | Notes |
|--------------|----------|-------|
| `user_id`    | yes | Permanent provider id (Google `sub`, Telegram id, Solana pubkey). |
| `app_token`  | yes | Must be registered in auth-center's `APP_TOKENS`. |
| `method`     | no\* | Real provider: `google`/`telegram`/`solana`. Defaults to `delegate` if omitted — send it so the receiver shows the right provider. |
| `name`       | no | Google-style display name. |
| `first_name` | no | Telegram/Solana-style. |
| `last_name`  | no | Telegram/Solana-style. |

\* Not enforced by the server, but always send it.

Send the fields that match the original provider:
- **Google:** `method: "google"` + `name`
- **Telegram:** `method: "telegram"` + `first_name` (+ `last_name`)
- **Solana:** `method: "solana"` (+ `first_name` if you have a label)

> If you store a single collapsed `name`, send it as both `name` and `first_name`. The
> receiver picks the first matching field, so there's no duplication — this just covers
> both Google- and Telegram-style renderers.

## Checklist to rebuild in a new app

- [ ] Avatar trigger + popover in a `position: relative` wrapper (`.profile-area`).
- [ ] Avatar shows the first letter of the name (JS reads `data-name`).
- [ ] Toggle visibility with an `.open` class on both popover and trigger; close on
      outside click.
- [ ] Identity block: provider (muted, uppercase), name, permanent id.
- [ ] **No** member-since / last-login rows — identity + actions only.
- [ ] Grouped action list `.pi-list`: container border + `overflow:hidden`, rows divided
      by `.pi-action + .pi-action { border-top }`, no per-row borders.
- [ ] Rows: **apps**, **readme**, **log out** (in that order).
- [ ] **log out stays red** and is the destructive action (clear session → login screen).
- [ ] "apps" → server `/apps` that forwards `user_id` + `app_token` + `method` + name
      fields to `/delegate`, then redirects to `https://menu.sh-development.ru/?code=<code>`.

## Client-rendered variant (JS SPA)

If your app isn't server-rendered, populate the identity block from a session endpoint
instead of template fields. Everything else (HTML classes, CSS, toggle JS) is identical.

```js
const user = await (await fetch('/me', { credentials: 'include' })).json();
profileBtn.dataset.name                              = user.name || user.id;
document.querySelector('.pi-label').textContent      = (user.method || '').toUpperCase();
document.querySelector('.pi-name').textContent       = user.name || user.id;
document.querySelector('.pi-sub').textContent        = user.name ? user.id : '';
// then run the same initial/toggle code from "Behavior (JS)" above
```
