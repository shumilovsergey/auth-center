# Auth-Center API

Centralized authentication API in Go with Ed25519 signed tokens and SQLite storage.

## Quick Start

1. Download binary
```bash
wget https://github.com/shumilovsergey/auth-center/blob/main/bin/auth-center
chmod +x auth-center
sudo mkdir -p /bin/auth && sudo mv auth-center /bin/auth/auth
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
WorkingDirectory=/bin/auth
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



## Build

### Requirements

- Go 1.21+
- GCC (for SQLite CGO compilation)

### Build

```bash
cd auth-center
go mod tidy
cd build && make build
# Binary: bin/auth-center
```

### Commands

```bash
make build   # Compile binary
make run     # Build and run
make clean   # Remove binary
make tidy    # Clean up go.mod
```

### Local Development (.env)

When running locally (without systemd), create `.env` in the project root:

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

