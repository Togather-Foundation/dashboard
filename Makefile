.PHONY: run build test vet fmt tidy

run:
	go run ./cmd/bff

build:
	go build -o bin/bff ./cmd/bff

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

tidy:
	go mod tidy

check: fmt vet test
