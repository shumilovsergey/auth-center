# Web Rules

Style and structure rules for apps built on this template.

---

## File Structure

```
web/
  favicon.svg   — browser tab icon (SVG)
  index.html    — Go template: shell + app markup
  shell.css     — shell styles: navbar, login screen, shared base
  shell.js      — shell logic: navbar, profile popover, chevron toggle
  app.css       — app-specific styles  ← edit this
  app.js        — app-specific logic   ← edit this
```

Register all files as explicit routes in `main.go`:
```go
mux.Handle("GET /favicon.svg", fileServer)
mux.Handle("GET /shell.css",   fileServer)
mux.Handle("GET /shell.js",    fileServer)
mux.Handle("GET /app.css",     fileServer)
mux.Handle("GET /app.js",      fileServer)
```

**Shell files** (`shell.css`, `shell.js`) are shared infrastructure — do not edit them per-app.  
**App files** (`app.css`, `app.js`) are yours. Add all app-specific styles and logic there.

---

## Layout

Single layout for all screen sizes — no breakpoints between mobile and desktop.

The 768px threshold is reserved in the codebase for future layout differences if needed, but currently unused.

### Not logged in — Login screen

```
          ┌─────────────────────┐
          │   APP-NAME          │  ← .login-logo (uppercase)
          │   sh-development.ru │  ← .login-domain (link to sh-development.ru)
          │                     │
          │   [ войти ]         │  ← .login-btn → /login
          └─────────────────────┘
                  ( ↓ )          ← .chevron-btn (toggles description)

    Description text in Russian  ← .app-about (hidden by default)
```

- `.login-card`: centered, max-width 380px, `border: 1.5px solid var(--border-active)`
- `.app-about`: always visible, `margin-top: 64px` places it at the same Y as the old expandable layout

### Logged in — Navbar + content

```
[ 1 ][ 2 ][ 3 ]                            [ 4 ]
 ←── .nav-tabs (pill) ──→          ←── .profile-area ──→
```

- `.navbar`: static at the top of the page — scrolls away when content is long
- Profile popover opens downward (`top: calc(100% + 10px)`)

---

## index.html Template Structure

```html
{{if .User}}
  <!-- navbar + app content -->
  <nav class="navbar"> ... </nav>
  <main class="app-content"> ... </main>
{{else}}
  <!-- login screen -->
  <div class="login-screen"> ... </div>
{{end}}
```

App content goes inside `<main class="app-content">`. The login screen is rendered server-side — no JS auth check needed.

---

## Navigation Tabs

Tabs are numbered `1`, `2`, `3` in the template. Replace with labels in real apps. Active tab switching is handled in `shell.js`. App-specific tab content switching goes in `app.js`.

---

## Profile Button

- **Logged in**: shows user's first initial (taken from `data-name` by `shell.js`). Clicking toggles popover with user info, app links, logout.
- **Not logged in**: not shown (login screen is used instead).

---

## Visual Style

Use CSS variables from `shell.css` — do not hardcode colors in `app.css`:

| Variable | Usage |
|---|---|
| `--bg` | Page background |
| `--card` | Card / popover surfaces |
| `--border` | Default borders |
| `--border-active` | Focused / active / primary borders |
| `--text` | Primary text |
| `--text-dim` | Secondary / label text |
| `--accent` | Highlighted values, active tab |
| `--neon` | Bright accent (titles) |

---

## Per-app Customisation Checklist

When building a new app on this template:

1. Set `.login-logo` text to the app name
2. Write `.app-about` description in Russian
3. Replace tab labels (`1 / 2 / 3`) with real section names
4. Replace `/open-food-scanner` in the popover with real app links (or remove it)
5. Add styles to `app.css`, logic to `app.js`
