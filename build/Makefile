.PHONY: build run clean

BINARY_NAME=auth-center
BINARY_PATH=../bin/$(BINARY_NAME)
MAIN_PATH=../cmd/server/main.go

build:
	cd .. && go build -o bin/$(BINARY_NAME) ./cmd/server

run: build
	cd .. && ./bin/$(BINARY_NAME)

clean:
	rm -f $(BINARY_PATH)

deps:
	cd .. && go mod download

tidy:
	cd .. && go mod tidy
