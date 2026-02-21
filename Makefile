.PHONY: lint test test_race

lint:
	go fix ./...
	golangci-lint run ./... --fix

test:
	go tool gotestsum -- -cover ./...

test_race:
	go tool gotestsum -- -race ./...

test_e2e:
	RUN_E2E_TESTS=true go tool gotestsum -- -cover ./...
