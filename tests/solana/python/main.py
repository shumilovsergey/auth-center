import os
import base64
import secrets
import nacl.signing
import nacl.exceptions
import base58
from flask import Flask, render_template, request, jsonify
from dotenv import load_dotenv

load_dotenv()

app = Flask(
    __name__,
    template_folder=os.path.join(os.path.dirname(__file__), "web"),
    static_folder=os.path.join(os.path.dirname(__file__), "web"),
    static_url_path="",
)

# in-memory nonce store: { public_key: nonce }
# MVP only — no DB, no TTL
nonces = {}


@app.route("/")
def index():
    return render_template("index.html")


@app.route("/nonce", methods=["POST"])
def get_nonce():
    data = request.get_json()
    public_key = (data or {}).get("public_key", "").strip()
    if not public_key:
        return jsonify({"error": "missing public_key"}), 400

    nonce = f"Sign in to MyApp\nNonce: {secrets.token_hex(16)}"
    nonces[public_key] = nonce
    return jsonify({"nonce": nonce})


@app.route("/auth", methods=["POST"])
def auth():
    data = request.get_json()
    if not data:
        return jsonify({"error": "no data"}), 400

    public_key = data.get("public_key", "").strip()
    signature_b64 = data.get("signature", "").strip()
    nonce = data.get("nonce", "").strip()

    if not all([public_key, signature_b64, nonce]):
        return jsonify({"error": "missing fields"}), 400

    # verify nonce matches what we issued
    if nonces.get(public_key) != nonce:
        return jsonify({"error": "invalid or expired nonce"}), 403

    try:
        pub_key_bytes = base58.b58decode(public_key)
        sig_bytes = base64.b64decode(signature_b64)
        message_bytes = nonce.encode()

        verify_key = nacl.signing.VerifyKey(pub_key_bytes)
        verify_key.verify(message_bytes, sig_bytes)
    except nacl.exceptions.BadSignatureError:
        return jsonify({"error": "invalid signature"}), 403
    except Exception:
        return jsonify({"error": "verification error"}), 400

    # one-time nonce — delete after use
    del nonces[public_key]

    return jsonify({"ok": True, "public_key": public_key})


if __name__ == "__main__":
    app.run(host="0.0.0.0", port=5000, debug=True)
