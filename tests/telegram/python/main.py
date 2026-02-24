import os
import hashlib
import hmac
from flask import Flask, render_template, request, jsonify
from dotenv import load_dotenv

load_dotenv()

BOT_TOKEN = os.getenv("BOT_TOKEN", "")
BOT_USERNAME = os.getenv("BOT_USERNAME", "")

app = Flask(
    __name__,
    template_folder=os.path.join(os.path.dirname(__file__), "web"),
    static_folder=os.path.join(os.path.dirname(__file__), "web"),
    static_url_path="",
)


@app.route("/")
def index():
    return render_template("index.html", bot_username=BOT_USERNAME)


@app.route("/auth", methods=["POST"])
def auth():
    data = request.get_json()
    if not data:
        return jsonify({"error": "no data"}), 400

    received_hash = data.pop("hash", None)
    if not received_hash:
        return jsonify({"error": "missing hash"}), 400

    # build data-check-string: sorted key=value pairs joined by \n
    data_check_string = "\n".join(f"{k}={v}" for k, v in sorted(data.items()))

    # secret key = SHA-256 of bot token
    secret_key = hashlib.sha256(BOT_TOKEN.encode()).digest()

    # expected hash = HMAC-SHA256 of data_check_string with secret_key
    expected_hash = hmac.new(
        secret_key, data_check_string.encode(), hashlib.sha256
    ).hexdigest()

    if not hmac.compare_digest(expected_hash, received_hash):
        return jsonify({"error": "invalid auth"}), 403

    return jsonify({"ok": True, "user": data})


if __name__ == "__main__":
    app.run(host="0.0.0.0", port=5000, debug=True)
