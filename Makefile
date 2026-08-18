.PHONY: test lint lint-fix check

test:
	go test ./... -race -count=1

lint:
	golangci-lint run ./...

lint-fix:
	golangci-lint run ./... --fix

check: lint-fix lint test
