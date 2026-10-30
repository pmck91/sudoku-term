.PHONY: build test check run

build:
	go build -buildvcs=false -o sudoku ./cmd/sudoku

test:
	go test -race ./...

check:
	go vet ./...
	@test -z "$$(gofmt -l cmd internal)"

run:
	go run ./cmd/sudoku
