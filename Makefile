.PHONY: build test check run

VERSION ?= dev

build:
	go build -buildvcs=false -ldflags "-X main.version=$(VERSION)" -o sudoku ./cmd/sudoku

test:
	go test -race ./...

check:
	go vet ./...
	@test -z "$$(gofmt -l cmd internal)"

run:
	go run ./cmd/sudoku
