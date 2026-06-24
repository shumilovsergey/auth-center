# Docker setup — reusable spec

How the Docker stuff works in wgetbash, so it can be recreated in another Go app.
Four things worth keeping:

1. **Dev**: hot-reload (Air), source mounted, runs natively on macOS ARM.
2. **Prod**: clean static binary cross-compiled for Linux/Intel (amd64).
3. **One multi-stage Dockerfile** for both.
4. **`--version` baked at build time** and printed by the prod build.

The trick that makes it simple: **one Dockerfile with two independent stages**
(`dev` and `builder`), and two compose files that each `target:` one stage.

---

## File map

| File | Role |
|------|------|
| `Dockerfile` | Multi-stage: `dev` (Air) + `builder` (static binary). |
| `dev-compose.yml` | Runs the `dev` stage, mounts source, hot-reload. |
| `prod-compose.yml` | Runs the `builder` stage, copies the binary out to `./bin`, prints a version banner. |
| `build/.air.toml` | Air config (what to watch, how to rebuild). |
| `.env` / `.env.example` | Runtime env (used by dev via `env_file`). |

---

## 1) Dev — hot-reload, mounted, native on macOS ARM

```bash
docker compose -f dev-compose.yml up --build
# http://localhost:8000
```

**Why it's nice on Apple Silicon:** the `dev` stage has **no `--platform`**, so it builds
and runs as the host's native arch (arm64 on M-series). No emulation, fast rebuilds.

### Dev stage (Dockerfile)

```dockerfile
# ── dev: hot-reload with Air ──
FROM golang:1.23-bookworm AS dev
WORKDIR /app
RUN go install github.com/air-verse/air@v1.52.3
ENV GONOSUMDB=* GOFLAGS=-mod=mod
EXPOSE 8000
CMD ["air"]
```

- `golang:1.23-bookworm` (Debian) — full toolchain, friendly for dev.
- `air` installed at a pinned version → reproducible.
- `GOFLAGS=-mod=mod` lets Go update go.mod/go.sum inside the container while you work.
- No `COPY` of source — the source comes in via a **bind mount** at runtime.

### Dev compose

```yaml
services:
  wgetbash:
    build:
      context: .
      target: dev          # <-- pick the dev stage
    ports:
      - "8000:8000"
    env_file: .env          # runtime config
    volumes:
      - ./build:/app        # source bind mount → edits on host trigger rebuilds
      - ./data:/data        # persisted DB / data
      - go-cache:/root/go/pkg/mod   # named volume: cache modules between runs

volumes:
  go-cache:
```

Three mounts, three jobs:
- `./build:/app` — **source mount**. Edit a file on the Mac, Air sees it in the container.
- `./data:/data` — **persistence**. DB survives container restarts (`DB_PATH=/data/...`).
- `go-cache` — **module cache**. Named volume so `go mod download` isn't repeated.

### Air config (`build/.air.toml`)

```toml
root = "."
tmp_dir = "tmp"

[build]
  cmd = "mkdir -p /tmp/airbuild && go build -o /tmp/airbuild/wgetbash ."
  bin = "/tmp/airbuild/wgetbash"
  include_ext = ["go", "js", "css", "html", "svg"]   # rebuild on these
  exclude_dir = ["bin", "data", "tmp"]               # ignore these
  delay = 200                                         # debounce (ms)

[misc]
  clean_on_exit = true
```

- Watches backend **and** frontend assets (`js/css/html/svg`) — because they're
  `//go:embed`-ed into the binary, a frontend edit needs a rebuild to show up.
- Builds to `/tmp/airbuild` (outside the mounted `/app`) so the binary never lands
  back on the host or retriggers a watch loop.

---

## 2) Prod — clean static binary for Linux/Intel

```bash
docker compose -f prod-compose.yml build --no-cache && docker compose -f prod-compose.yml up
# Output: ./bin/wgetbash  (static linux/amd64)
```

### Builder stage (Dockerfile)

```dockerfile
# ── builder: compiles linux/amd64 static binary ──
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY build/ .
RUN BUILD_TIME=$(date -u +%Y-%m-%dT%H:%M:%SZ) && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w -X main.buildTime=${BUILD_TIME}" -o /wgetbash .
```

What makes the binary "clean" and portable:
- `CGO_ENABLED=0` → **fully static**, no libc dependency. Runs on any Linux, in a
  `scratch`/`distroless` image, or bare on a server as a systemd unit. (Requires a
  pure-Go SQLite driver like `modernc.org/sqlite`; `mattn/go-sqlite3` would break this.)
- `GOOS=linux GOARCH=amd64` → **cross-compiles for Intel** even when building on an
  ARM Mac. Cross-compile is free here precisely because CGO is off.
- `-ldflags="-s -w"` → strip symbol/debug tables → smaller binary.
- Unlike dev, the builder **`COPY`s** the source in (no mount) — reproducible, hermetic.

### Prod compose — extract the binary + version banner

```yaml
services:
  release:
    build:
      context: .
      target: builder        # <-- pick the builder stage
    volumes:
      - ./bin:/out           # binary gets copied here on the host
    entrypoint:
      - /bin/sh
      - -c
      - |
        cp /wgetbash /out/wgetbash
        printf "\033[1;32m\n"
        printf "  ╔══════════════════════════════════╗\n"
        printf "  ║        WGETBASH PROD BUILD       ║\n"
        printf "  ║  Built: %-26s║\n" "$(/out/wgetbash --version)"
        printf "  ╚══════════════════════════════════╝\n"
        printf "\033[0m\n"
```

The pattern: the image's only job is to **produce a binary**. The compose service runs
it once, copies the binary to a host-mounted `./bin`, and prints a green banner that
calls `--version` to confirm what was built. There's no long-running server here — it
builds, reports, exits.

---

## 3) The Dockerfile (whole thing)

Two stages, no shared base, each self-contained. Compose chooses one via `target:`.

```dockerfile
# ── dev: hot-reload with Air ──
FROM golang:1.23-bookworm AS dev
WORKDIR /app
RUN go install github.com/air-verse/air@v1.52.3
ENV GONOSUMDB=* GOFLAGS=-mod=mod
EXPOSE 8000
CMD ["air"]

# ── builder: compiles linux/amd64 static binary ──
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY build/ .
RUN BUILD_TIME=$(date -u +%Y-%m-%dT%H:%M:%SZ) && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w -X main.buildTime=${BUILD_TIME}" -o /wgetbash .
```

Why two compose files instead of one with profiles: each workflow is a single
`docker compose -f <file> up`, and `target:` keeps the stages from interfering.

---

## 4) `--version` — build time baked into the binary

The build stamps a UTC timestamp into a Go variable via linker flags; the binary prints
it on `--version`.

### In the build

```
-ldflags="-X main.buildTime=${BUILD_TIME}"
```

`-X importpath.name=value` sets a **string variable at link time** — no source edit, no
config file. `BUILD_TIME` is generated with `date -u +%Y-%m-%dT%H:%M:%SZ`.

### In the code (`main.go`)

```go
var buildTime = "dev"   // default when run outside the build (e.g. `go run`)

func main() {
    if len(os.Args) > 1 && os.Args[1] == "--version" {
        fmt.Println(buildTime)
        return
    }
    // ... normal startup ...
}
```

- Declare a package-level `var` with a fallback (`"dev"`) so local runs still work.
- The `--version` flag short-circuits before any server startup.
- The prod compose banner calls `$(/out/wgetbash --version)` to echo it back — instant
  confirmation that the freshly built binary carries the right timestamp.

---

## Runtime env

Dev reads `.env` via `env_file`. Keep a committed `.env.example` as the template
(`.env` itself is gitignored):

```
PORT=8000
DB_PATH=/data/wgetbash.db      # points at the mounted ./data volume
AUTH_URL=...
AUTH_INTERNAL=...
APP_URL=http://localhost:8000
APP_TOKEN=...
SECRET_KEY=...
```

The prod binary is configured the same way in production — on the server it runs as a
systemd unit with `Environment=` lines (see `build/wgetbash.service.example`),
`Restart=always`.

---

## Checklist to rebuild in a new app

- [ ] One multi-stage `Dockerfile`: a `dev` stage (Air, no platform pin → native ARM)
      and a `builder` stage (`CGO_ENABLED=0 GOOS=linux GOARCH=amd64`, ldflags strip).
- [ ] `dev-compose.yml` → `target: dev`, mount source + data, named module cache,
      `env_file: .env`.
- [ ] `prod-compose.yml` → `target: builder`, mount `./bin:/out`, copy binary out,
      print a `--version` banner.
- [ ] `.air.toml` watching backend **and** embedded frontend assets, building outside
      the mounted dir.
- [ ] `var buildTime = "dev"` + `--version` flag; stamp via `-ldflags "-X main.buildTime=..."`.
- [ ] Pure-Go deps only (e.g. `modernc.org/sqlite`) so the static cross-compile holds.
- [ ] Commit `.env.example`, gitignore `.env`, `bin/`, `data/`, `*.db`.
```
