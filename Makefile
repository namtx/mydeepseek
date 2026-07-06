BINARY := fake-ollama

.PHONY: build run test vet clean install

build:
	go build -o $(BINARY) .

run:
	go run .

test:
	go test ./...

vet:
	go vet ./...

clean:
	rm -f $(BINARY)

install:
	go install .
