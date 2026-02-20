.PHONY: lint test test_race

lint:
	go fix ./...
	golangci-lint run ./... --fix

test:
	go tool gotestsum -- -cover ./...

test_race:
	go tool gotestsum -- -race ./...
