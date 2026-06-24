# Background — fast-loading blurred backdrop

How the full-screen background in this app loads fast, and how to reproduce it in
future apps built on the same template.

## Why it's fast

The background is **only ~7.5 KB** (`build/web/background.webp`, 1280×936). Four things
make it load instantly:

1. **It's blurred in CSS, not baked into the image.** Because `#bg` applies
   `filter: blur(40px)`, the source image never needs to be sharp or high-res. A small,
   heavily-compressed WebP looks identical once blurred — so we trade resolution we can't
   see for bytes we don't download.
2. **WebP format.** ~25–35 % smaller than the equivalent JPEG at the same quality.
3. **Preloaded with high priority** in `<head>` so the browser fetches it before CSS even
   asks for it.
4. **Embedded in the Go binary + cached for 30 days** — no disk read, no re-download on
   repeat visits.

## The four pieces

### 1. The CSS layer (`web/app.css`)

A single fixed full-viewport `div`, pushed behind everything and blurred:

```css
html { background: #000; }   /* fallback while the image loads */
body { background: transparent; }

#bg {
  position: fixed;
  inset: -10%;                              /* overscan so blur edges aren't visible */
  background-image: url('/background.webp');
  background-size: cover;
  background-position: center;
  filter: blur(40px) brightness(0.75);      /* the blur — lets the source be low-res */
  transform: scale(1.1);                    /* hide blur bleed at the edges */
  z-index: -1;
}
```

### 2. The markup (`web/index.html`)

The empty div, first thing in `<body>`:

```html
<body>
<div id="bg"></div>
```

And the preload hint in `<head>` — this is the single biggest perceived-speed win:

```html
<link rel="preload" href="/background.webp" as="image" fetchpriority="high" />
```

### 3. Embedding + routing (`build/main.go`)

The whole `web/` folder is embedded into the binary:

```go
//go:embed web
var webFiles embed.FS
```

…and the image is served with a long cache header so browsers never refetch it:

```go
mux.Handle("GET /background.webp", cacheStatic(fileServer))
```

```go
// cacheStatic wraps a handler with a 30-day immutable cache header.
func cacheStatic(h http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Cache-Control", "public, max-age=2592000") // 30 days
        h.ServeHTTP(w, r)
    })
}
```

## Checklist for a new app

1. Drop `background.webp` into `web/`.
2. Add the `#bg` rules to `app.css` (the blur is what keeps it cheap).
3. Add `<div id="bg"></div>` as the first element in `<body>`.
4. Add the `<link rel="preload" ... fetchpriority="high">` to `<head>`.
5. Register `GET /background.webp` with `cacheStatic(fileServer)` in `main.go`.

That's it — no per-request work, no big download.

## Making the WebP on macOS

Because CSS blurs the image, **downscale aggressively first** — there's no point shipping
4K when it gets blurred to mush. ~1280px wide is plenty; smaller is fine.

### Option A — `cwebp` (Google's encoder, recommended)

```bash
brew install webp
```

```bash
# Resize to ~1280px wide and encode. -q is quality 0–100; 60–75 is great for a blurred bg.
cwebp -q 70 -resize 1280 0 input.jpg -o background.webp
```

`-resize 1280 0` sets width to 1280 and keeps the aspect ratio (0 = auto height).
Drop `-q` lower (e.g. `-q 50`) to shrink further — you won't see it through the blur.

### Option B — `sips` (built into macOS, no install)

Recent macOS ships WebP support in `sips`:

```bash
# Resize to 1280px on the longest side, then convert to webp.
sips -Z 1280 input.jpg --out resized.jpg
sips -s format webp resized.jpg --out background.webp
```

If your macOS `sips` doesn't list `webp` (`sips --help | grep -i webp`), use Option A.

### Option C — ImageMagick

```bash
brew install imagemagick
magick input.jpg -resize 1280x -quality 70 background.webp
```

### Sanity check

```bash
ls -la background.webp          # aim for well under ~50 KB; ours is ~7.5 KB
file background.webp            # should say: Web/P image, VP8 encoding, WxH
```

Aim for the smallest file that still looks good **after** the CSS blur — start at `-q 70`,
go lower until you see banding through the blur, then step back up one notch.
