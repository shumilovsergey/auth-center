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
from flask import Flask, render_template, request, jsonify
from dotenv import load_dotenv

load_dotenv()

BOT_TOKEN = os.getenv("BOT_TOKEN", "")
BOT_USERNAME = os.getenv("BOT_USERNAME", "")
WEBHOOK_SECRET = os.getenv("WEBHOOK_SECRET", "")

app = Flask(
    __name__,
    template_folder=os.path.join(os.path.dirname(__file__), "web"),
    static_folder=os.path.join(os.path.dirname(__file__), "web"),
    static_url_path="",
)

# --- Telegram QR sessions: { token: { status, user, created_at } }
sessions = {}
SESSION_TTL = 300  # 5 minutes

# --- Solana nonces: { public_key: nonce }
nonces = {}


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
    return render_template("index.html")


# ── telegram QR ─────────────────────────────────────────────────────────────

@app.route("/qr-session", methods=["POST"])
def qr_session():
    cleanup_sessions()
    token = secrets.token_urlsafe(32)
    sessions[token] = {"status": "pending", "user": None, "created_at": time.time()}
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
    return jsonify({"status": session["status"], "user": session.get("user")})


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
                session["status"] = "authenticated"
                session["user"] = {
                    "id": from_user.get("id"),
                    "first_name": from_user.get("first_name"),
                    "last_name": from_user.get("last_name"),
                    "username": from_user.get("username"),
                }
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
    return jsonify({"ok": True, "public_key": public_key})


if __name__ == "__main__":
    app.run(host="0.0.0.0", port=5000, debug=True)
