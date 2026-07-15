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

---

# Nginx and `X-Forwarded-For` — Security Note

## TL;DR

If nginx is the **only proxy** in front of an application that trusts
`X-Forwarded-For`, **never** use:

```nginx
proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
```

Use:

```nginx
proxy_set_header X-Forwarded-For $remote_addr;
```

---

# Why?

Applications often determine the client's IP from the first value in
`X-Forwarded-For`.

Example application logic:

```go
clientIP := strings.Split(xff, ",")[0]
```

This is common because in a trusted proxy chain the first IP is expected to be
the original client.

---

# The problem with `$proxy_add_x_forwarded_for`

This variable **appends** nginx's client IP to whatever the client already sent.

If the client sends:

```
X-Forwarded-For: 1.2.3.4
```

nginx forwards:

```
X-Forwarded-For: 1.2.3.4, 178.66.128.220
```

If the application trusts the **first** IP, it now believes the client is
`1.2.3.4`.

The client completely controls that value.

This allows IP whitelist bypasses.

Example:

```
Allowed IP:
5.189.254.175

Attacker:
203.0.113.50

Request:

X-Forwarded-For: 5.189.254.175
```

nginx forwards:

```
X-Forwarded-For:
5.189.254.175,203.0.113.50
```

Application reads:

```
5.189.254.175
```

Access granted.

---

# Correct configuration

If nginx is the first and only reverse proxy:

```nginx
proxy_set_header X-Forwarded-For $remote_addr;
```

Now regardless of what the client sends:

```
Client:

X-Forwarded-For: 5.189.254.175
```

auth-proxy receives:

```
X-Forwarded-For: 203.0.113.50
```

The spoofed value is discarded.

---

# When is `$proxy_add_x_forwarded_for` correct?

Only when nginx itself trusts the proxy in front of it.

Typical example:

```
Internet
     │
Cloudflare
     │
nginx
     │
Application
```

or

```
Internet
     │
Load Balancer
     │
nginx
     │
Application
```

In these cases nginx must first be configured with the `real_ip` module:

```nginx
set_real_ip_from <trusted proxy>;
real_ip_header X-Forwarded-For;
```

After that:

```nginx
$remote_addr
```

already contains the real client IP.

Only then does forwarding or extending `X-Forwarded-For` make sense.

---

# Rule of thumb

## Single nginx in front of the app

```
proxy_set_header X-Forwarded-For $remote_addr;
```

✅ Safe

---

## Multiple trusted proxies

Configure `real_ip` first.

Only after that should `X-Forwarded-For` be propagated.

---

## Never trust client-supplied X-Forwarded-For

Treat it exactly like any other HTTP header:

```
User-Agent
Cookie
Referer
X-Forwarded-For
```

The client can send whatever they want.

It only becomes trustworthy after passing through infrastructure that you own
and explicitly trust.

---

# Security impact

This mistake can bypass:

- IP allowlists
- Admin panels
- Internal APIs
- GitHub proxy restrictions
- Deployment endpoints
- Monitoring endpoints
- Any authentication based on client IP

It is a surprisingly common reverse proxy misconfiguration.