# Navbar — segmented pill tabs

A floating "pill" bar holding 2–4 tabs. One tab is active at a time. The whole
bar is a dark translucent capsule with a frosted-glass blur; the active tab is a
lighter capsule that sits *inside* it. Used as the primary in-app navigation.

Two interchangeable variants, **same markup, same JS, same behaviour** — only a
few CSS values differ:

1. **Text** — wide pills with a label (`text 1`, `text 2`, …).
2. **Icons** — round circles with a single glyph/icon each. Saves horizontal space.

---

## What it looks like

**Text variant** (current)

```
╭───────────────────────────────────────────────────────────╮
│  ╭───────────╮                                             │
│  │  text 1   │     text 2          text 3                  │
│  ╰───────────╯                                             │
╰───────────────────────────────────────────────────────────╯
   ↑ active = lighter filled capsule, bright text
                ↑ inactive = no fill, dimmed text (brightens on hover)
```

**Icon variant**

```
╭───────────────────────────╮
│  ╭───╮                     │
│  │ ◉ │   ○      ○          │
│  ╰───╯                     │
╰───────────────────────────╯
   ↑ active circle filled      ↑ inactive circles, dimmed
```

### Design rules
- **Outer bar:** dark translucent capsule (`rgba(0,0,0,0.38)`), 1px hairline
  border, fully rounded ends, `backdrop-filter: blur(24px) saturate(1.6)` so the
  background shows through frosted.
- **Tabs:** transparent by default, dimmed text/icon (40% white). On hover they
  brighten to 75%. The active tab gets a subtle white fill (12% white) and bright
  text/icon (92% white).
- **One active at a time.** Clicking a tab moves the active state to it.
- **Transitions** are quick (`.15s`) on background + colour only.
- Everything is built on translucency — it must sit on top of a photo/gradient
  background to look right (the blur is the whole point).

---

## Markup (identical for both variants)

```html
<nav class="navbar">
  <div class="nav-tabs">
    <button class="tab active">text 1</button>
    <button class="tab">text 2</button>
    <button class="tab">text 3</button>
  </div>
</nav>
```

For the **icon variant**, put an icon/glyph inside instead of a label (SVG, an
icon-font `<i>`, or just an emoji/character):

```html
<nav class="navbar">
  <div class="nav-tabs">
    <button class="tab active" aria-label="Home">⌂</button>
    <button class="tab" aria-label="Search">⌕</button>
    <button class="tab" aria-label="Profile">☺</button>
  </div>
</nav>
```

> Add `aria-label` on icon buttons so they stay accessible without a text label.

---

## CSS

Base styles are shared. The two variants differ **only** in the `.tab` sizing
rules noted below.

```css
.navbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 16px;
}

/* the outer frosted capsule that holds the tabs */
.nav-tabs {
  display: flex;
  align-items: center;
  background: rgba(0, 0, 0, 0.38);
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 28px;
  padding: 5px;
  gap: 2px;
  backdrop-filter: blur(24px) saturate(1.6);
  -webkit-backdrop-filter: blur(24px) saturate(1.6);
}

.tab {
  background: transparent;
  border: none;
  border-radius: 20px;
  height: 44px;
  min-width: 44px;
  color: rgba(255, 255, 255, 0.40);
  font-family: inherit;
  font-size: 14px;
  white-space: nowrap;
  cursor: pointer;
  transition: background .15s, color .15s;
}

.tab:hover  { color: rgba(255, 255, 255, 0.75); }
.tab.active { background: rgba(255, 255, 255, 0.12); color: rgba(255, 255, 255, 0.92); }
```

### Variant 1 — Text (default)

```css
.tab { padding: 0 18px; }   /* horizontal padding gives each pill its width */
```

### Variant 2 — Icons (circles)

```css
.tab {
  padding: 0;
  width: 44px;          /* equal width + height = a circle */
  border-radius: 50%;
  font-size: 18px;      /* bump the glyph a touch */
}
```

That's the only difference. `height: 44px` + `width: 44px` + `border-radius:
50%` turns each pill into a circle; dropping the horizontal padding is what saves
the space.

---

## Behaviour (JS)

Pure class-toggling — no framework. Clicking a tab clears `.active` from all tabs
and sets it on the clicked one. Works for both variants unchanged.

```js
document.querySelectorAll('.tab').forEach(tab => {
  tab.addEventListener('click', () => {
    document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
    tab.classList.add('active');
  });
});
```

To also switch the page content per tab, give each tab a `data-target` and show
the matching panel:

```html
<button class="tab active" data-target="panel1">text 1</button>
...
<section id="panel1">...</section>
```
```js
document.querySelectorAll('.tab').forEach(tab => {
  tab.addEventListener('click', () => {
    document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
    tab.classList.add('active');
    document.querySelectorAll('[id^="panel"]').forEach(p => p.hidden = true);
    document.getElementById(tab.dataset.target).hidden = false;
  });
});
```

---

## Notes for the developer

- Keep **2–4 tabs**. More than that and the icon variant is the better choice.
- The bar relies on `backdrop-filter` — make sure there's a real background
  behind it (image/gradient), otherwise the frosted look disappears.
- Colours are plain `rgba(255,255,255,α)` whites over a dark bar; if you have a
  theme palette, swap them for your variables but keep the **opacity steps**
  (≈0.40 idle → 0.75 hover → 0.92 active) — that contrast ladder is what makes
  the active state read clearly.
- Sizes: `44px` tap target is intentional (comfortable on mobile). Scale the
  whole thing by changing `height`/`min-width` and the radii together.
