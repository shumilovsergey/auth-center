# ── dev: runs the service locally ────────────────────────────────────────
FROM golang:1.23-alpine AS dev
WORKDIR /app
COPY build/ .
RUN go mod tidy
EXPOSE 8886
CMD ["go", "run", "."]

# ── builder: compiles linux/amd64 binary ─────────────────────────────────
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY build/ .
RUN go mod tidy && \
    BUILD_TIME=$(date -u +%Y-%m-%dT%H:%M:%SZ) && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w -X main.buildTime=${BUILD_TIME}" \
    -o /auth-center .
