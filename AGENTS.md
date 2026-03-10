# AGENTS.md

## Project Overview

Bidirectional sync between Jira and Todoist. Tasks in current or future Jira sprints are synced to Todoist sections that mirror Jira statuses; changes on either side propagate to the other based on timestamps.

- Entry point: `main.go` -> `cmd/` (Cobra CLI)
- Two modes: `sync` (one-shot) and `watch` (polling on an interval)

## Project Structure

- `cmd/` — CLI commands (`root.go`, `sync.go`, `watch.go`)
- `config/` — Config loading via Viper, status mapping helpers
- `syncer/` — Core sync engine (`engine.go`) and link helpers (`link.go`)
- `jira/` — Jira REST client (`client.go`), types (`types.go`), ADF-to-Markdown conversion (`adf.go`)
- `todoist/` — Todoist REST client (`client.go`), types (`types.go`)

## Coding Conventions

- Error wrapping: `fmt.Errorf("context: %w", err)`
- Logging: zerolog with structured fields — `Str()`, `Err()`, `Int()`, `Msg()`
- No interfaces; concrete types with constructor injection (`NewEngine(...)`, `NewClient(...)`)
- Line length: 120 chars max (enforced by golines)
- Naming: `ctx` for context, `cfg` for config, `req`/`resp` for request/response

## Configuration

- Loaded from `.env` and environment variables via Viper
- Secrets (`TODOIST_TOKEN`, `JIRA_TOKEN`, etc.) live in `.env` — never commit this file
- Status mapping: `config.StatusMap` maps Jira statuses to Todoist sections; use `JiraToTodoistStatus()` / `TodoistToJiraStatus()` for lookups

## Lint and Test

After making changes, run:

```sh
make lint
make test
```

Fix any resulting issues. **If tests fail, don't modify the test unless it is absolutely necessary due to changes in the underlying code.**

`make test_e2e` runs E2E tests against real Jira/Todoist APIs and requires `RUN_E2E_TESTS=true` plus valid credentials in `.env`.

## Testing Patterns

- Table-driven tests with `testify/assert` and `testify/require`
- Unit tests use `t.Parallel()`
- E2E tests are guarded by `RUN_E2E_TESTS=true` and use `t.Cleanup()` for resource teardown

## Key Architecture

- **Sync flow:** fetch both sides in parallel (`errgroup`) -> link by Jira key in Todoist task name -> create/update/sync whichever side is newer
- **Sprint filtering:** only tasks in current or future sprints are synced; tasks removed from all sprints are deleted from Todoist
- **Comment sync:** one-way Jira -> Todoist, tagged with `jira-comment:` markers to avoid duplicates
- **Failed transitions:** if a Jira transition fails (e.g. "Blocked" requires a reason), the Todoist task is reverted to match Jira and a comment is added explaining the failure
