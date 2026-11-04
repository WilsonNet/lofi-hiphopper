BINARY := bin/lofi-hiphopper

.PHONY: build run demo test race vet clean

build:
	go build -o $(BINARY) .

run:
	go run .

demo:
	go run . -demo

test:
	go test ./...

race:
	go test -race ./...

vet:
	go vet ./...

clean:
	rm -rf bin
