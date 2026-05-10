# Build & Deploy

## Local dev

Hot-reload via `go run`:
```bash
docker-compose up auth
```

Force rebuild (after adding a dependency or changing the Dockerfile):
```bash
docker-compose up --build auth
```

## Production binary (linux/amd64)

Build and copy binary to `bin/`:
```bash
docker-compose run --rm release
```

Force rebuild from scratch (no Docker cache):
```bash
docker-compose build --no-cache release && docker-compose run --rm release
```

