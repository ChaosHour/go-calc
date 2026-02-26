.PHONY: build run clean

build:
	@mkdir -p bin
	go build -o bin/go-calc ./cmd/calc

run: build
	./bin/go-calc -cpu 24

clean:
	rm -rf bin
