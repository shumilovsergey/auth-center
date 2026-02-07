# Auth-Center API

Centralized authentication API in Go with Ed25519 signed tokens and SQLite storage.

## Quick Start

1. Download binary
```bash
wget https://raw.githubusercontent.com/shumilovsergey/auth-center/main/bin/auth-center
```
```
chmod +x auth-center
```
```
sudo mkdir -p /bin/auth-center && sudo mv auth-center /bin/auth-center/auth-center
```

2. Create systemd service
```bash
sudo nano /etc/systemd/system/auth-center.service
```
```ini
[Unit]
Description=Auth-Center API
After=network.target

[Service]
Type=simple
User=srv
Group=srv
WorkingDirectory=/bin/auth-center
ExecStart=/bin/auth-center/auth-center
Restart=always
RestartSec=5
Environment=PORT=9103
Environment=TOKENS=your-secret-token-1,your-secret-token-2
Environment=DB_PATH=/bin/auth-center/auth-center.db

[Install]
WantedBy=multi-user.target
```

3. Set ownership
```bash
sudo chown -R srv:srv /bin/auth-center
```

4. Start service
```bash
sudo systemctl daemon-reload
sudo systemctl enable auth-center
sudo systemctl start auth-center
sudo systemctl status auth-center
```

5. Health check
```bash
curl http://localhost:9103/health
```

## Development

Requires Docker. All source code and build files live in `build/`.

### Run locally (dev mode)
```bash
cd build && make run
```
Starts the server inside a container with source code mounted. Code changes require restart.

### Build Linux binary
```bash
cd build && make build
```
Outputs a statically linked Linux x86-64 binary to `bin/auth-center`.

### All commands (run from `build/`)
```bash
make build   # Build Linux binary via Docker
make run     # Run dev server in Docker
make clean   # Remove binary
make tidy    # go mod tidy
make deps    # go mod download
```

### Local .env

Create `.env` in the project root:

```env
PORT=8080
TOKENS=token1,token2,token3
DB_PATH=./auth-center.db
```

## API Endpoints

All endpoints require `X-Auth-Token` header with a valid backend token.

### POST /register

Create a new user.

**Request:**
```json
{"username": "user", "password": "pass"}
```

**Response (success):**
```json
{"status": "created", "public_key": "base64-token..."}
```

**Response (user already exists):**
```json
{"error": "user already exists"}
```
Status code: `409 Conflict`

---

### POST /login

Authenticate an existing user.

**Request:**
```json
{"username": "user", "password": "pass"}
```

**Response (success):**
```json
{"status": "authenticated", "public_key": "base64-token..."}
```

**Response (wrong password or user not found):**
```json
{"status": "invalid", "public_key": ""}
```

---

### POST /verify

Check if a token is valid for a user. Returns user data on success.

**Request:**
```json
{"username": "user", "public_key": "base64-token..."}
```

**Response (valid):**
```json
{
  "valid": true,
  "user_id": 123,
  "username": "user",
  "created_at": "2024-01-15T10:30:00Z"
}
```

**Response (invalid):**
```json
{"valid": false}
```

---

### POST /refresh

Generate new token using a valid old one.

**Request:**
```json
{"username": "user", "old_public_key": "base64-token..."}
```

**Response:**
```json
{"public_key": "base64-new-token..."}
```

---

### Error Responses

Missing token header:
```json
{"error": "missing X-Auth-Token header"}
```

Invalid token:
```json
{"error": "invalid token"}
```
