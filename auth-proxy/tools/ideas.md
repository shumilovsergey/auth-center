# auth-proxy — ideas / open design notes

> Scratchpad for a future feature. **Nothing here is built yet.** Telegram routes
> (`/webhook`, `/tg-api/{path...}`) are out of scope and stay untouched.

## The idea: a generic app forward route

Sometimes (not most of the time) an app in the ecosystem needs to send an outbound
request *through* the proxy — because the proxy sits on a network that can reach things
the app's own VPS can't (same reason `/tg-api` exists). We want a **default transport**
that covers the common 90% case, with anything weird (auth keys, signed headers,
per-service quirks) split off into its own special handler later.

### Decisions reached so far

1. **One global route, not route-per-app.**
   Onboarding a new app should need *zero* proxy changes. A single `POST /forward`
   keeps the proxy as dumb as `/tg-api` already is — it doesn't "know" about apps.
   Per-app identity comes from *which token matched*, not from separate routes.

2. **Don't store redirect URLs on the proxy — but DO store a destination boundary.**
   The caller names the destination in a header (stateless, flexible). What the proxy
   stores is not URLs but an *allowlist of permitted hosts* + an always-on block of
   private/loopback/link-local ranges. This is the line between a handy egress relay
   and an authenticated SSRF / open-relay machine.

3. **Proxy holds its own tokens (`PROXY_TOKENS` env).**
   auth-center has **no** "is this token valid" endpoint and **no** A-for-B trust check
   — its `APP_TOKENS` model is flat ("all apps mutually trusted", see
   `auth-center/tools/delegate.md`). So nothing can be offloaded; the proxy validates
   tokens itself. Prefer `name:secret` pairs so logs can name the app.

4. **Default transport = body + Content-Type only.**
   No copying the caller's `Authorization`/arbitrary headers. Cases that need upstream
   auth keys are *special* → their own dedicated route later (like `/tg-api` is its own
   thing). Rule of thumb: if the upstream wants the secret in the **body**, the calling
   app puts it there and it just rides through the default path; if the upstream demands
   it in a **header**, that's the signal it belongs in a special handler, not here.

### Proposed request contract (draft)

```
POST /forward
X-App-Token:  <secret>            # who — constant-time match against PROXY_TOKENS
X-Forward-To: <destination URL>   # where
Content-Type: ...
<body streamed through>
```

Handler logic: (1) match token → app name, reject unknown; (2) parse `X-Forward-To`,
check host against allowlist + private-IP block (resolve DNS first, to stop rebinding);
(3) stream body to destination, stream response back. Never log the token; log
`app=<name> → <host>` for abuse visibility.

### Security floor (non-negotiable if this ships)

- Host allowlist via env, e.g. `FORWARD_ALLOW=api.telegram.org,api.example.com`.
- Always block `127/8`, `10/8`, `172.16/12`, `192.168/16`, `::1`, `169.254/16`
  (cloud metadata!) — even for allowlisted hostnames, after DNS resolution.
- Constant-time token compare; never log the token.
- Consider: body-size cap, request timeout, per-token rate limit.

### Still open / to decide later

- **Allowlist granularity:** one global allowlist (simplest, lean toward this for v1)
  vs. per-app (`menu` may hit Telegram, `wgetbash` may not).
- **Header passthrough policy:** confirmed default = none beyond Content-Type; revisit
  only when a real special case appears.
- Whether response headers should be copied back, or just status + body (current
  Telegram forwarders copy only status + body).

### Scope when we build it

Purely additive: a new `forward.go` + one route registration. No changes to the
Telegram path. Keep it thin.
