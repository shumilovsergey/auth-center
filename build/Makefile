.PHONY: build run clean deps tidy

BINARY_NAME=auth-center
BINARY_PATH=../bin/$(BINARY_NAME)

build:
	docker compose run --rm build

run:
	docker compose up dev

clean:
	rm -f $(BINARY_PATH)

deps:
	go mod download

tidy:
	go mod tidy
