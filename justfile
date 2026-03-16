# Run all linting and fixes
lint:
    golangci-lint run ./... --fix

# Run standard tests with coverage
test:
    go tool gotestsum -- -cover ./...

# Run tests with race detection
test_race:
    go tool gotestsum -- -race ./...

# Run end-to-end tests with the required environment variable
test_e2e:
    export RUN_E2E_TESTS := "true"
    go tool gotestsum -- -cover ./...

install:
    ./install.sh
