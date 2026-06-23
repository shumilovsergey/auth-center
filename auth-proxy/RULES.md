# Build & Deploy

## Local dev

Hot-reload via `go run`:
```bash
docker-compose -f dev-compose.yml up proxy
```

Force rebuild (after adding a dependency or changing the Dockerfile):
```bash
docker-compose -f dev-compose.yml up --build proxy
```

## Production binary (linux/amd64)

Build and copy binary to `bin/`:
```bash
docker-compose -f prod-compose.yml run --rm release
```

Force rebuild from scratch (no Docker cache):
```bash
docker-compose -f prod-compose.yml build --no-cache release && docker-compose -f prod-compose.yml run --rm release
```

