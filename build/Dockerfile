FROM golang:1.21

RUN apt-get update && apt-get install -y gcc sqlite3 && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o /app/bin/auth-center ./cmd/server

EXPOSE 8080

CMD ["/app/bin/auth-center"]
