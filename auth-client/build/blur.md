# Blur Background Recipe

## HTML

```html
<div id="scene">
  <div id="bg"></div>
  <div id="content">your content</div>
</div>
```

## CSS

```css
#scene {
  position: relative;
  overflow: hidden;
  background: #000;
}

#bg {
  position: absolute;
  inset: -10%;
  background-image: url('background.webp');
  background-size: cover;
  background-position: center;
  filter: blur(40px) brightness(0.9);
  transform: scale(1.1);
}

#content {
  position: relative;
  z-index: 1;
}
```

## Why each line

- `inset: -10%` — the div overflows its parent so blurred edges stay hidden
- `transform: scale(1.1)` — extra margin against edge bleed
- `overflow: hidden` on parent — clips the oversized div
- `background: #000` on parent — fallback color when no image is set

## Swap the image from JS

```js
document.getElementById('bg').style.backgroundImage = "url('background.webp')";
```

To clear it (show plain dark background):

```js
document.getElementById('bg').style.backgroundImage = '';
```

---

# Login Window

The login card floats over the blurred background with a glassy frosted look.

## HTML structure

```html
<div class="login-screen">
  <div id="login-bg"></div>           <!-- blurred bg image, same recipe as above -->
  <div class="login-card-wrap">
    <div class="login-card">
      <span class="login-logo">blur</span>
      <a class="login-domain">domain.com</a>
      <a class="login-btn">Sign in with …</a>
    </div>
  </div>
  <p class="app-about">…description…</p>
</div>
```

## CSS

```css
/* Screen: image background + dark overlay */
.login-screen {
  position: relative;
  overflow: hidden;
  background: #000;
}

#login-bg {
  position: absolute;
  inset: -10%;
  background-size: cover;
  background-position: center;
  filter: blur(40px) brightness(0.9) saturate(1);
  transform: scale(1.1);
  z-index: 0;
}

.login-screen::before {
  content: '';
  position: absolute;
  inset: 0;
  background: rgba(0, 0, 0, 0.35);
  z-index: 1;
  pointer-events: none;
}

.login-card-wrap { position: relative; z-index: 2; }

/* Glassy card */
.login-card {
  background: rgba(0, 0, 0, 0.38);
  border: 1px solid rgba(255, 255, 255, 0.15);
  backdrop-filter: blur(28px) saturate(1.6);
  -webkit-backdrop-filter: blur(28px) saturate(1.6);
  border-radius: 16px;
  padding: 44px 40px 36px;
  max-width: 380px;
}

/* Main text — light white */
.login-logo {
  color: rgba(255, 255, 255, 0.92);
  text-shadow: 0 1px 4px rgba(0, 0, 0, 0.8);
}

/* Secondary text — dimmed grey */
.login-domain {
  color: rgba(255, 255, 255, 0.4);
}
.login-domain:hover { color: rgba(255, 255, 255, 0.75); }

/* Glassy login button */
.login-btn {
  background: rgba(255, 255, 255, 0.08);
  border: 1px solid rgba(255, 255, 255, 0.18);
  color: rgba(255, 255, 255, 0.88);   /* main light color */
  border-radius: 10px;
  padding: 14px;
}

.login-btn:hover {
  background: rgba(255, 255, 255, 0.16);
  border-color: rgba(255, 255, 255, 0.38);
}

/* Description below the card — secondary grey */
.app-about {
  color: rgba(255, 255, 255, 0.4);
  text-shadow: 0 1px 4px rgba(0, 0, 0, 0.8);
}
```

## Text color tokens

| Role | Value | Usage |
|---|---|---|
| Main text | `rgba(255, 255, 255, 0.92)` | Logo, button labels |
| Secondary text | `rgba(255, 255, 255, 0.40)` | Domain, description, dimmed labels |

---

# User Bar (Profile Button & Popover)

The profile button and its dropdown share the same glassy language as the login card.

## CSS

```css
/* Round profile avatar button — glassy */
.profile-btn {
  background: rgba(255, 255, 255, 0.12);
  border: 1px solid rgba(255, 255, 255, 0.50);
  color: rgba(255, 255, 255, 0.88);        /* main light */
  backdrop-filter: blur(24px) saturate(1.6);
  -webkit-backdrop-filter: blur(24px) saturate(1.6);
  text-shadow: 0 1px 4px rgba(0, 0, 0, 0.8), 0 2px 12px rgba(0, 0, 0, 0.4);
  border-radius: 50%;
  width: 54px;
  height: 54px;
}

.profile-btn:hover,
.profile-btn.open { border-color: rgba(255, 255, 255, 0.80); color: rgba(255, 255, 255, 1); }

/* Popover dropdown — glassy panel */
.profile-popover {
  background: rgba(0, 0, 0, 0.42);
  border: 1px solid rgba(255, 255, 255, 0.12);
  backdrop-filter: blur(24px) saturate(1.6);
  -webkit-backdrop-filter: blur(24px) saturate(1.6);
  border-radius: 16px;
}

/* Popover text */
.pi-label { color: rgba(255, 255, 255, 0.45); }   /* secondary grey: tiny uppercase label */
.pi-name  { color: rgba(255, 255, 255, 0.92); }   /* main light: user name */
.pi-sub   { color: rgba(255, 255, 255, 0.45); }   /* secondary grey: sub-info */

/* Action buttons inside popover — glassy */
.pi-action {
  border: 1px solid rgba(255, 255, 255, 0.50);
  color: rgba(255, 255, 255, 0.88);
  border-radius: 10px;
  padding: 9px 12px;
}
.pi-action:hover {
  border-color: rgba(255, 255, 255, 0.80);
  color: rgba(255, 255, 255, 1);
}

/* Logout — red variant */
.pi-logout {
  color: #ff9f9f;
  border-color: #a03030;
}
.pi-logout:hover {
  color: #ffbfbf;
  border-color: #cc4040;
  background: rgba(255, 80, 80, 0.12);
}
```

## Glassy panel recipe (reusable)

Any panel that should look glassy over the blurred background:

```css
.glass-panel {
  background: rgba(0, 0, 0, 0.38–0.42);     /* darker = more opaque */
  border: 1px solid rgba(255, 255, 255, 0.12–0.18);
  backdrop-filter: blur(24–28px) saturate(1.6);
  -webkit-backdrop-filter: blur(24–28px) saturate(1.6);
  border-radius: 12–16px;
}
```