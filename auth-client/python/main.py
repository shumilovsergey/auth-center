import os
import requests as http
from flask import Flask, request, session, redirect, render_template_string, Response
from dotenv import load_dotenv

load_dotenv()

AUTH_URL    = os.getenv("AUTH_URL",    "http://localhost:8886")   # public (browser)
AUTH_INTERNAL = os.getenv("AUTH_INTERNAL", "http://localhost:8886")  # internal (server→server)
APP_URL     = os.getenv("APP_URL",     "http://localhost:8890")
SECRET_KEY  = os.getenv("SECRET_KEY",  "dev-secret")
APP_TOKEN   = os.getenv("APP_TOKEN",   "")

app = Flask(__name__)
app.secret_key = SECRET_KEY


# ── pages ────────────────────────────────────────────────────────────────────

HOME = """<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>auth</title>
  <link rel="icon" href="/favicon.svg" type="image/svg+xml" />
  <style>
    :root {
      --bg: #0c0c12; --card: #111120; --border: #252340;
      --border-active: #7c68d0; --text: #b0b0cc; --text-dim: #42405e;
      --accent: #a78bfa; --neon: #c4b5fd;
    }
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      font-family: ui-monospace, 'Cascadia Code', 'Fira Code', monospace;
      font-size: 13px;
      background:
        radial-gradient(ellipse at 50% 0%,   #1a1830 0%, transparent 65%),
        radial-gradient(ellipse at 100% 100%, #0f0e1e 0%, transparent 60%),
        #080810;
      min-height: 100vh;
      display: flex; align-items: center; justify-content: center;
      padding: 24px;
    }
    .card {
      background: var(--card); border: 1px solid var(--border);
      border-radius: 16px; padding: 28px; width: 100%; max-width: 380px;
      display: flex; flex-direction: column; gap: 16px;
    }
    .title {
      border: 1px solid var(--border-active); border-radius: 10px;
      padding: 13px 18px; text-align: center;
      color: var(--neon); letter-spacing: 0.12em;
    }
    .row {
      border: 1px solid var(--border); border-radius: 10px;
      padding: 13px 18px; color: var(--text-dim);
      font-size: 12px; line-height: 1.8;
    }
    .row .key   { color: var(--text-dim); font-size: 11px; letter-spacing: 0.08em; }
    .row .value { color: var(--accent); word-break: break-all; }
    .btn {
      display: block; width: 100%; background: var(--bg);
      border: 1px solid var(--border); border-radius: 10px;
      padding: 13px 18px; color: var(--text-dim); font-family: inherit;
      font-size: 13px; letter-spacing: 0.05em; text-align: center;
      text-decoration: none; cursor: pointer;
      transition: border-color .2s, color .2s;
    }
    .btn:hover { border-color: var(--border-active); color: var(--text); }
    .btn.primary {
      border-color: var(--border-active); color: var(--accent);
    }
    .btn.primary:hover { border-color: var(--neon); color: var(--neon); }
    .error {
      border: 1px solid #5c2e2e; border-radius: 10px;
      padding: 13px 18px; color: #e87a7a; font-size: 12px;
      background: rgba(70,30,30,.12);
    }
  </style>
</head>
<body>
<div class="card">
  <div class="title">auth</div>

  {% if error %}
  <div class="error">{{ error }}</div>
  {% endif %}

  {% if user %}
    <div class="row">
      <span class="key">method</span><br>
      <span class="value">{{ method }}</span>
    </div>

    {% if method == 'telegram' %}
    <div class="row">
      <span class="key">telegram id</span><br>
      <span class="value">{{ user.id }}</span>
      {% set name = [user.first_name, user.last_name] | select('string') | join(' ') %}
      {% if name %}
      <br><br><span class="key">name</span><br>
      <span class="value">{{ name }}</span>
      {% endif %}
      {% if user.username %}
      <br><br><span class="key">username</span><br>
      <span class="value">@{{ user.username }}</span>
      {% endif %}
    </div>
    {% endif %}

    {% if method == 'solana' %}
    <div class="row">
      <span class="key">wallet</span><br>
      <span class="value">{{ user.id }}</span>
    </div>
    {% endif %}

    {% if method == 'google' %}
    <div class="row">
      <span class="key">google id</span><br>
      <span class="value">{{ user.id }}</span>
      {% if user.email %}
      <br><br><span class="key">email</span><br>
      <span class="value">{{ user.email }}</span>
      {% endif %}
      {% if user.name %}
      <br><br><span class="key">name</span><br>
      <span class="value">{{ user.name }}</span>
      {% endif %}
    </div>
    {% endif %}

    <a class="btn" href="/logout">log out</a>

  {% else %}
    <div class="row">
      <span class="key">status</span><br>
      <span class="value">not authenticated</span>
    </div>
    <a class="btn primary" href="/login">log in</a>
  {% endif %}
</div>
</body>
</html>
"""


FAVICON = '''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">
  <rect width="32" height="32" rx="8" fill="#111120"/>
  <circle cx="11" cy="16" r="5" fill="none" stroke="#c4b5fd" stroke-width="2"/>
  <path d="M16 16H27" stroke="#c4b5fd" stroke-width="2" stroke-linecap="round"/>
  <path d="M22 16V21" stroke="#c4b5fd" stroke-width="2" stroke-linecap="round"/>
  <path d="M26 16V19" stroke="#c4b5fd" stroke-width="2" stroke-linecap="round"/>
</svg>'''


@app.route("/favicon.svg")
def favicon():
    return Response(FAVICON, mimetype="image/svg+xml")


@app.route("/")
def index():
    # ── handle incoming code from auth center ──
    code = request.args.get("code")
    error = None

    if code:
        try:
            resp = http.post(
                f"{AUTH_INTERNAL}/exchange",
                json={"code": code, "app_token": APP_TOKEN},
                timeout=5,
            )
            data = resp.json()
            if data.get("ok"):
                session["user"]   = data["user"]
                session["method"] = data["method"]
                return redirect("/")
            else:
                error = data.get("error", "exchange failed")
        except Exception as e:
            error = f"could not reach auth center: {e}"

    user   = session.get("user")
    method = session.get("method")
    return render_template_string(HOME, user=user, method=method, error=error)


@app.route("/login")
def login():
    return redirect(f"{AUTH_URL}/?redirect={APP_URL}/")


@app.route("/logout")
def logout():
    session.clear()
    return redirect("/")


if __name__ == "__main__":
    app.run(host="0.0.0.0", port=8890, debug=True)
