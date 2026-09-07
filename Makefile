.PHONY: check build

check:
	go test -race ./...
	go vet ./...

build:
	go build -o bin/agent-commons ./cmd/agent-commons
