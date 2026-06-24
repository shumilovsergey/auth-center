# User Menu — reusable spec

A self-contained spec for the avatar → dropdown "user menu" used in wgetbash, so it
can be rebuilt in another app. Focus is on **structure, the buttons, and what they
do** — not exact colors. The one color rule worth keeping: **log out is red.**

## What it is

A circular avatar button in the top nav. Clicking it toggles a popover anchored to the
button. The popover has two parts:

1. **Identity block** (read-only) — provider label, display name, permanent user id.
2. **Action list** — a modern grouped list (rows divided by hairlines, no per-row
   borders) with: **apps**, **readme**, **log out**.

```
┌──────────────────────────────┐
│ GOOGLE                        │  ← provider (uppercase, muted)
│ Сергей Шумилов                │  ← display name
│ 105993223363753461270         │  ← permanent user id (auth provider id)
│ ┌──────────────────────────┐  │
│ │ apps                     │  │  ← delegates to another app (see below)
│ ├──────────────────────────┤  │  ← hairline divider between rows
│ │ readme                   │  │  ← external link, new tab
│ ├──────────────────────────┤  │
│ │ log out                  │  │  ← RED text/hover, clears session
│ └──────────────────────────┘  │
└──────────────────────────────┘
```

## Structure (HTML)

```html
<div class="profile-area">
  <!-- avatar trigger: shows the first letter of the user's name -->
  <button class="profile-btn" id="userTrig" title="menu">
    <span id="userInitial">U</span>
  </button>

  <!-- popover -->
  <div class="profile-popover" id="userDrop">
    <!-- identity block: filled in from /auth/me -->
    <div class="pi-info">
      <span class="pi-label" id="userProvider"></span>  <!-- e.g. GOOGLE -->
      <span class="pi-name"  id="userName"></span>      <!-- display name -->
      <span class="pi-sub"   id="userUid"></span>       <!-- permanent id -->
    </div>

    <!-- grouped action list -->
    <div class="pi-list">
      <a class="pi-action" href="/auth/delegate">apps</a>
      <a class="pi-action" href="https://github.com/.../README.md"
         target="_blank" rel="noopener">readme</a>
      <button class="pi-action pi-logout" id="logoutRow">log out</button>
    </div>
  </div>
</div>
```

Key points:
- Trigger and popover live in the same positioned wrapper (`.profile-area`,
  `position: relative`) so the popover can anchor to the button.
- Action rows can be `<a>` (navigation/link) or `<button>` (JS action) — they share the
  `.pi-action` class so they look identical.
- `pi-logout` is the only row with its own color treatment (red).

## The buttons — what each one does

| Button   | Element              | Action |
|----------|----------------------|--------|
| **apps** | `<a href="/auth/delegate">` | Full-page navigation to the app's delegate endpoint. The server delegates the logged-in identity to another app and redirects there (see "Delegation" below). |
| **readme** | `<a target="_blank">` | Opens the project README in a new tab. Pure link, no JS. |
| **log out** | `<button id="logoutRow">` | `POST /auth/logout` (clears the session cookie), then reloads the page → user lands back on the login screen. **Red** to mark it as the destructive action. |

## Behavior (JS)

```js
// Toggle the popover; close any other open dropdowns first.
$('userTrig').addEventListener('click', e => {
  e.stopPropagation();
  $('grpDrop').style.display = 'none';            // close sibling menus
  const open = $('userDrop').classList.toggle('open');
  $('userTrig').classList.toggle('open', open);   // keep trigger in sync (hover/active style)
});

// Click outside the menu → close it.
document.addEventListener('click', e => {
  if (!e.target.closest('.profile-area')) /* close all dropdowns */;
});

// Populate identity from the session endpoint.
const user = await (await fetch('/auth/me', { credentials: 'include' })).json();
$('userInitial').textContent  = init(user.username); // first letter for the avatar
$('userName').textContent     = user.username;
$('userUid').textContent      = user.uid || '';
$('userProvider').textContent = user.provider || '';

// Log out.
$('logoutRow').addEventListener('click', async () => {
  await fetch('/auth/logout', { method: 'POST', credentials: 'include' });
  window.location.reload();
});
```

Open/close is driven entirely by an `.open` class toggled on both the popover and the
trigger. `display: none` by default; `.open` switches it to `display: flex`.

## Styling rules that matter (color-agnostic)

What makes it look "modern list" rather than "stack of buttons":

```css
/* popover: floating card anchored under the trigger */
.profile-popover {
  position: absolute; top: calc(100% + 10px); right: 0;
  width: 256px; border-radius: 16px; padding: 14px;
  display: none; flex-direction: column; gap: 8px;
  /* translucent + blur for the glass look; no box-shadow (kept flat) */
  backdrop-filter: blur(24px) saturate(1.6);
}
.profile-popover.open { display: flex; }

/* identity block */
.pi-info  { display: flex; flex-direction: column; gap: 3px; }
.pi-label { font-size: 10px; letter-spacing: .1em; text-transform: uppercase; } /* muted */
.pi-name  { font-size: 15px; word-break: break-all; }
.pi-sub   { font-size: 11px; word-break: break-all; }                            /* muted */

/* grouped action list: ONE rounded box, rows divided by hairlines */
.pi-list {
  display: flex; flex-direction: column;
  border: 1px solid rgba(255,255,255,.12);
  border-radius: 12px; overflow: hidden;        /* clip rows to the rounded corners */
}
.pi-action {
  display: block; width: 100%; text-align: left;
  padding: 11px 14px; border: none; border-radius: 0;
  background: transparent; text-decoration: none;
  font: inherit; font-size: 13px; cursor: pointer;
  transition: background .15s, color .15s;
}
/* divider only BETWEEN rows, never on the first one */
.pi-action + .pi-action { border-top: 1px solid rgba(255,255,255,.10); }
.pi-action:hover { background: rgba(255,255,255,.06); }

/* the one intentional color: log out is red */
.pi-logout       { color: #ff9f9f; }
.pi-logout:hover { color: #ffbfbf; background: rgba(255,80,80,.12); }
```

The grouped-list recipe in three rules:
1. Wrap rows in a container with a border + `border-radius` + `overflow: hidden`.
2. Give each row no border of its own, only padding.
3. Add a top border to every row **except the first** (`.pi-action + .pi-action`).

## Delegation (the "apps" button)

"apps" hands the logged-in user to another app (app B) through a stateless auth-center,
passing the **name and provider** so app B shows the real name, not "no name".

### Server endpoint (`GET /auth/delegate`, behind auth)

1. Load the current user's permanent id, display name, and provider from the DB.
2. `POST {AUTH_INTERNAL}/delegate` with:
   ```json
   {
     "user_id":    "<permanent provider id>",
     "app_token":  "<this app's APP_TOKEN>",
     "method":     "google",            // the REAL provider, so app B shows it
     "name":       "Сергей Шумилов",    // Google-style display name
     "first_name": "Сергей Шумилов"     // Telegram/Solana-style fallback
   }
   ```
3. Read the returned one-time `code`.
4. `302` redirect the browser to `https://<app-B>/?code=<code>`.

App B then redeems the code via its own `POST /exchange` and renders the name.

### Why name + method must be forwarded

auth-center is **stateless** — it only forwards what the caller sends. If app A sends
just `user_id`, app B receives only the id and shows "no name". So app A must persist
the name it got at login and forward `method` + `name` (or `first_name`/`last_name`)
on every `/delegate` call.

| Field        | Required | Notes |
|--------------|----------|-------|
| `user_id`    | yes | Permanent provider id (Google `sub`, Telegram id, Solana pubkey). |
| `app_token`  | yes | Must be registered in auth-center's `APP_TOKENS`. |
| `method`     | no\* | Real provider: `google`/`telegram`/`solana`. Defaults to `delegate` if omitted — send it so app B shows the right provider. |
| `name`       | no | Google-style display name. |
| `first_name` | no | Telegram/Solana-style. |
| `last_name`  | no | Telegram/Solana-style. |

\* Not enforced by the server, but always send it.

Send the fields that match the original provider:
- **Google:** `method: "google"` + `name`
- **Telegram:** `method: "telegram"` + `first_name` (+ `last_name`)
- **Solana:** `method: "solana"` (+ `first_name` if you have a label)

> In wgetbash we store a single `username`, so we send it as both `name` and
> `first_name`. The receiver picks the first matching field, so there's no
> duplication — this just covers both Google- and Telegram-style renderers.

## Checklist to rebuild in a new app

- [ ] Avatar trigger + popover in a `position: relative` wrapper.
- [ ] Toggle visibility with an `.open` class; close on outside click.
- [ ] Identity block populated from your session/`/auth/me` (provider, name, id).
- [ ] Grouped action list: container border + `overflow:hidden`, rows divided by
      `.row + .row { border-top }`.
- [ ] **log out stays red** and is the destructive action (clear session → reload).
- [ ] "apps" → server `/auth/delegate` that forwards `user_id` + `app_token` +
      `method` + name fields, then redirects to app B with the returned `code`.
