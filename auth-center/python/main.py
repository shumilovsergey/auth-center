import os
import time
import secrets
import io
import base64
import qrcode
import requests as http
import nacl.signing
import nacl.exceptions
import base58
from urllib.parse import urlencode
from flask import Flask, render_template, request, jsonify, redirect
from dotenv import load_dotenv

load_dotenv()

BOT_TOKEN = os.getenv("BOT_TOKEN", "")
BOT_USERNAME = os.getenv("BOT_USERNAME", "")
WEBHOOK_SECRET = os.getenv("WEBHOOK_SECRET", "")
APP_TOKENS = set(t.strip() for t in os.getenv("APP_TOKENS", "").split(",") if t.strip())
DIRECT_REDIRECT = os.getenv("DIRECT_REDIRECT", "")
GOOGLE_CLIENT_ID = os.getenv("GOOGLE_CLIENT_ID", "")
GOOGLE_CLIENT_SECRET = os.getenv("GOOGLE_CLIENT_SECRET", "")
GOOGLE_CALLBACK_URL = os.getenv("GOOGLE_CALLBACK_URL", "http://localhost:5000/google/callback")

app = Flask(
    __name__,
    template_folder=os.path.join(os.path.dirname(__file__), "web"),
    static_folder=os.path.join(os.path.dirname(__file__), "web"),
    static_url_path="",
)

# --- Google OAuth states: { state: { redirect, created_at } }
google_states = {}
STATE_TTL = 300  # 5 minutes

# --- Telegram QR sessions: { token: { status, user, created_at, redirect } }
sessions = {}
SESSION_TTL = 300  # 5 minutes

# --- Solana nonces: { public_key: nonce }
nonces = {}

# --- One-time codes: { code: { user, method, created_at } }
one_time_codes = {}
CODE_TTL = 60  # 1 minute


# ── helpers ────────────────────────────────────────────────────────────────

def make_qr_b64(url: str) -> str:
    qr = qrcode.QRCode(border=2)
    qr.add_data(url)
    qr.make(fit=True)
    img = qr.make_image(fill_color="#c4b5fd", back_color="#08080f")
    buf = io.BytesIO()
    img.save(buf, format="PNG")
    return base64.b64encode(buf.getvalue()).decode()


def cleanup_sessions():
    now = time.time()
    expired = [k for k, v in sessions.items() if now - v["created_at"] > SESSION_TTL]
    for k in expired:
        del sessions[k]


def cleanup_codes():
    now = time.time()
    expired = [k for k, v in one_time_codes.items() if now - v["created_at"] > CODE_TTL]
    for k in expired:
        del one_time_codes[k]


def generate_code(user: dict, method: str) -> str:
    cleanup_codes()
    code = secrets.token_urlsafe(32)
    one_time_codes[code] = {"user": user, "method": method, "created_at": time.time()}
    return code


def build_redirect(url: str, code: str) -> str:
    sep = "&" if "?" in url else "?"
    return f"{url}{sep}code={code}"


def send_telegram_message(chat_id: int, text: str):
    try:
        http.post(
            f"https://api.telegram.org/bot{BOT_TOKEN}/sendMessage",
            json={"chat_id": chat_id, "text": text},
            timeout=5,
        )
    except Exception:
        pass


# ── main page ───────────────────────────────────────────────────────────────

@app.route("/")
def index():
    redirect_url = request.args.get("redirect", "")
    if not redirect_url and DIRECT_REDIRECT:
        return redirect(DIRECT_REDIRECT)
    return render_template("index.html", redirect_url=redirect_url)


# ── telegram QR ─────────────────────────────────────────────────────────────

@app.route("/qr-session", methods=["POST"])
def qr_session():
    cleanup_sessions()
    body = request.get_json(silent=True) or {}
    redirect_url = body.get("redirect", "")

    token = secrets.token_urlsafe(32)
    sessions[token] = {
        "status": "pending",
        "user": None,
        "created_at": time.time(),
        "redirect": redirect_url,
    }
    tme_url = f"https://t.me/{BOT_USERNAME}?start={token}"
    return jsonify({"token": token, "qr": make_qr_b64(tme_url), "url": tme_url})


@app.route("/poll/<token>", methods=["GET"])
def poll(token):
    session = sessions.get(token)
    if not session:
        return jsonify({"status": "expired"}), 404
    if time.time() - session["created_at"] > SESSION_TTL:
        del sessions[token]
        return jsonify({"status": "expired"}), 404

    resp = {"status": session["status"], "user": session.get("user")}

    if session["status"] == "authenticated" and session.get("redirect"):
        resp["code"] = session.get("code")
        resp["redirect"] = session["redirect"]

    return jsonify(resp)


@app.route("/webhook", methods=["POST"])
def webhook():
    if WEBHOOK_SECRET:
        if request.headers.get("X-Telegram-Bot-Api-Secret-Token") != WEBHOOK_SECRET:
            return "", 403

    update = request.get_json()
    if not update:
        return "", 400

    message = update.get("message", {})
    text = message.get("text", "")
    from_user = message.get("from", {})

    if text.startswith("/start "):
        token = text.split(" ", 1)[1].strip()
        session = sessions.get(token)
        if session and session["status"] == "pending":
            if time.time() - session["created_at"] <= SESSION_TTL:
                user = {
                    "id": from_user.get("id"),
                    "first_name": from_user.get("first_name"),
                    "last_name": from_user.get("last_name"),
                    "username": from_user.get("username"),
                }
                session["status"] = "authenticated"
                session["user"] = user

                if session.get("redirect"):
                    code = generate_code(user, "telegram")
                    session["code"] = code

                send_telegram_message(from_user["id"], "You are authenticated!")
            else:
                send_telegram_message(from_user["id"], "This QR code has expired.")

    return "", 200


# ── solana ──────────────────────────────────────────────────────────────────

@app.route("/solana/nonce", methods=["POST"])
def solana_nonce():
    data = request.get_json()
    public_key = (data or {}).get("public_key", "").strip()
    if not public_key:
        return jsonify({"error": "missing public_key"}), 400
    nonce = f"Sign in to Auth Center\nNonce: {secrets.token_hex(16)}"
    nonces[public_key] = nonce
    return jsonify({"nonce": nonce})


@app.route("/solana/auth", methods=["POST"])
def solana_auth():
    data = request.get_json()
    if not data:
        return jsonify({"error": "no data"}), 400

    public_key = data.get("public_key", "").strip()
    signature_b64 = data.get("signature", "").strip()
    nonce = data.get("nonce", "").strip()
    redirect_url = data.get("redirect", "")

    if not all([public_key, signature_b64, nonce]):
        return jsonify({"error": "missing fields"}), 400

    if nonces.get(public_key) != nonce:
        return jsonify({"error": "invalid or expired nonce"}), 403

    try:
        verify_key = nacl.signing.VerifyKey(base58.b58decode(public_key))
        verify_key.verify(nonce.encode(), base64.b64decode(signature_b64))
    except nacl.exceptions.BadSignatureError:
        return jsonify({"error": "invalid signature"}), 403
    except Exception:
        return jsonify({"error": "verification error"}), 400

    del nonces[public_key]

    resp = {"ok": True, "public_key": public_key}

    if redirect_url:
        user = {"id": public_key}
        code = generate_code(user, "solana")
        resp["code"] = code
        resp["redirect"] = redirect_url

    return jsonify(resp)


# ── google oauth ────────────────────────────────────────────────────────────

@app.route("/google/login")
def google_login():
    redirect_url = request.args.get("redirect", "")
    state = secrets.token_urlsafe(32)
    google_states[state] = {"redirect": redirect_url, "created_at": time.time()}

    params = urlencode({
        "client_id":     GOOGLE_CLIENT_ID,
        "redirect_uri":  GOOGLE_CALLBACK_URL,
        "response_type": "code",
        "scope":         "openid email profile",
        "state":         state,
    })
    return redirect(f"https://accounts.google.com/o/oauth2/v2/auth?{params}")


@app.route("/google/callback")
def google_callback():
    error = request.args.get("error")
    if error:
        return f"google auth error: {error}", 400

    state = request.args.get("state", "")
    code  = request.args.get("code", "")

    state_data = google_states.pop(state, None)
    if not state_data or time.time() - state_data["created_at"] > STATE_TTL:
        return "invalid or expired state", 400

    # exchange code for access token
    token_resp = http.post(
        "https://oauth2.googleapis.com/token",
        json={
            "code":          code,
            "client_id":     GOOGLE_CLIENT_ID,
            "client_secret": GOOGLE_CLIENT_SECRET,
            "redirect_uri":  GOOGLE_CALLBACK_URL,
            "grant_type":    "authorization_code",
        },
        timeout=10,
    ).json()

    if "error" in token_resp:
        return f"token error: {token_resp['error']}", 400

    # fetch user info
    user_resp = http.get(
        "https://www.googleapis.com/oauth2/v3/userinfo",
        headers={"Authorization": f"Bearer {token_resp['access_token']}"},
        timeout=10,
    ).json()

    user = {
        "id":    user_resp["sub"],
        "email": user_resp.get("email", ""),
        "name":  user_resp.get("name", ""),
    }

    redirect_url = state_data["redirect"]
    if redirect_url:
        code_val = generate_code(user, "google")
        return redirect(build_redirect(redirect_url, code_val))

    return redirect(DIRECT_REDIRECT or "/")


# ── code exchange ────────────────────────────────────────────────────────────

@app.route("/exchange", methods=["POST"])
def exchange():
    cleanup_codes()
    data = request.get_json()
    if not data:
        return jsonify({"error": "no data"}), 400

    if APP_TOKENS and data.get("app_token") not in APP_TOKENS:
        return jsonify({"error": "unauthorized"}), 403

    code = data.get("code", "").strip()
    if not code:
        return jsonify({"error": "missing code"}), 400

    entry = one_time_codes.get(code)
    if not entry:
        return jsonify({"error": "invalid or expired code"}), 403

    if time.time() - entry["created_at"] > CODE_TTL:
        del one_time_codes[code]
        return jsonify({"error": "code expired"}), 403

    del one_time_codes[code]
    return jsonify({"ok": True, "user": entry["user"], "method": entry["method"]})


if __name__ == "__main__":
    app.run(host="0.0.0.0", port=5000, debug=True)
