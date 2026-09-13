.PHONY: test build deploy

test:
	go test ./...

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/orchestrator ./cmd/orchestrator

deploy:
	./scripts/deploy.sh
