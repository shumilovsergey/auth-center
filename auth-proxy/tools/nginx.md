# Auth Proxy — nginx setup for the GitHub proxy

The `/github/*` routes let a box that **can't reach GitHub** (Ansible/Semaphore,
deploy scripts) pull through auth-proxy, which sits on a host that *can*. It's a
true reverse proxy — the box talks only to auth-proxy, and auth-proxy fetches from
GitHub and streams the bytes back.

Access is gated by client IP (`GITHUB_ALLOWED_IPS`). The IP is read from the
**`X-Forwarded-For`** header — so nginx must set it, and set it *safely*.

## ⚠️ The one thing that must be right: X-Forwarded-For

The IP gate reads the **first** entry of `X-Forwarded-For`. If nginx *appends* to
a client-supplied header (the common `$proxy_add_x_forwarded_for` default), a
caller can spoof `X-Forwarded-For: <an-allowed-ip>` and walk straight through the
gate.

**Overwrite it with the real TCP peer — do not append:**

```nginx
proxy_set_header X-Forwarded-For $remote_addr;   # ✅ overwrites, discards spoofed value
# proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;  # ❌ appends — spoofable
```

(If auth-proxy is ever behind *another* trusted proxy/CDN, this changes — use
nginx's `real_ip` module to derive the true client. For a single nginx in front,
`$remote_addr` is correct.)

## Location block

Add this to the auth-proxy `server { }` block. It overwrites XFF, lifts the body
size limit (git and binaries can be large), disables buffering so downloads
stream, and raises timeouts for slow clones/downloads.

```nginx
location /github/ {
    proxy_pass http://127.0.0.1:8080;

    proxy_set_header Host              $host;
    proxy_set_header X-Forwarded-For   $remote_addr;   # security-critical (see above)
    proxy_set_header X-Forwarded-Proto $scheme;

    client_max_body_size    0;      # git-upload-pack / large binaries
    proxy_request_buffering off;     # stream request body straight through
    proxy_buffering         off;     # stream response body straight back

    proxy_read_timeout      300s;    # slow clone / big download
    proxy_send_timeout      300s;
}
```

The rest of the routes (`/webhook`, `/tg-api/*`, `/health`, …) keep whatever
`location / { proxy_pass http://127.0.0.1:8080; }` you already have.

## Enable the feature on auth-proxy

The `/github/*` routes are **off** until you set the allowed IPs. In
`bin/auth-proxy.service`:

```ini
Environment=GITHUB_ALLOWED_IPS=<semaphore/deploy box IP>
```

Comma-separated for more than one box. Then `systemctl restart auth-proxy`.
Until it's set, every `/github/*` request returns `403`.

## URL examples

Rule of thumb: replace `https://github.com/` with
`https://auth-proxy.sh-development.ru/github/` and leave the rest of the path
as-is. GitHub 302-redirects `.../raw/...` to `raw.githubusercontent.com`, and the
proxy follows that redirect server-side, so the box never needs to reach GitHub.

**Ansible / Semaphore repo** — https mode, Access Key = `None` (public repo);
drop the `git@`:

```
git@github.com:shumilovsergey/semaphore.git
→ https://auth-proxy.sh-development.ru/github/shumilovsergey/semaphore.git
```

**Binary downloads (host-swap):**

```
https://github.com/shumilovsergey/wgetbash/raw/refs/heads/main/bin/wgetbash
→ https://auth-proxy.sh-development.ru/github/shumilovsergey/wgetbash/raw/refs/heads/main/bin/wgetbash

https://github.com/shumilovsergey/blur-4/raw/refs/heads/main/bin/blur
→ https://auth-proxy.sh-development.ru/github/shumilovsergey/blur-4/raw/refs/heads/main/bin/blur

https://github.com/shumilovsergey/nom-nom/raw/refs/heads/main/bin/nom-nom
→ https://auth-proxy.sh-development.ru/github/shumilovsergey/nom-nom/raw/refs/heads/main/bin/nom-nom
```

**Optional — direct-to-raw variant** (one hop fewer; hits
`raw.githubusercontent.com` directly). Note the `/raw/` moves to right after
`/github/` and disappears from the middle:

```
https://auth-proxy.sh-development.ru/github/raw/shumilovsergey/wgetbash/refs/heads/main/bin/wgetbash
https://auth-proxy.sh-development.ru/github/raw/shumilovsergey/blur-4/refs/heads/main/bin/blur
https://auth-proxy.sh-development.ru/github/raw/shumilovsergey/nom-nom/refs/heads/main/bin/nom-nom
```

## Routing recap

| Prefix on the proxy | Upstream | Used by |
|---|---|---|
| `/github/raw/{owner}/{repo}/{ref}/{path}` | `raw.githubusercontent.com/...` | binary downloads (direct) |
| `/github/{owner}/{repo}.git/...` | `github.com/...` | `git pull` (Ansible) |
| `/github/{owner}/{repo}/raw/{ref}/{path}` | `github.com/...` → 302 → `raw.githubusercontent.com` | binary downloads (host-swap) |
