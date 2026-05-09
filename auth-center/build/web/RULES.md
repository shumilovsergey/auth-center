# Web Rules

Style and structure rules for the auth-center UI and any app built on this template.

## Structure

Every app must have these four files in `web/`:

| File | Purpose |
|---|---|
| `favicon.svg` | Browser tab icon — SVG so it scales at any size |
| `index.html` | Markup and Go template logic only — no inline styles or scripts |
| `style.css` | All styles |
| `script.js` | All client-side logic |

Register each file as an explicit route in `main.go`:
```go
mux.Handle("GET /favicon.svg", fileServer)
mux.Handle("GET /style.css",   fileServer)
mux.Handle("GET /script.js",   fileServer)
```

## Visual Style

Use the CSS variables defined in `style.css` — do not hardcode colors.
