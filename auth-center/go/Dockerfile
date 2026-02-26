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
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /binary .
