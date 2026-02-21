# todoist-jira-sync

A CLI tool for bidirectional synchronization between Todoist and Jira Cloud.

## Features

- **Task creation**: New Todoist tasks are created as Jira issues and vice versa
- **Descriptions & comments**: Synced bidirectionally with attribution prefixes
- **Status mapping**: Todoist sections map to Jira workflow statuses
- **Due dates**: Kept in sync on both sides
- **Conflict resolution**: Most recently updated side wins (uses Jira's `updated` timestamp)
- **No external state**: Cross-links are embedded in task/issue descriptions

## Installation

```bash
go install github.com/kalverra/todoist-jira-sync@latest
```

## Configuration

Configuration can be provided via CLI flags, environment variables, or a `.env` file in the working directory (loaded automatically via [godotenv](https://github.com/joho/godotenv)).

### Required Settings

| Setting | Env Var | Flag |
|---|---|---|
| Todoist API token | `TODOIST_API_TOKEN` | `--todoist-token` |
| Todoist project name | `TODOIST_PROJECT` | `--todoist-project` |
| Jira base URL | `JIRA_URL` | `--jira-url` |
| Jira email | `JIRA_EMAIL` | `--jira-email` |
| Jira API token | `JIRA_API_TOKEN` | `--jira-token` |
| Jira project key | `JIRA_PROJECT` | `--jira-project` |
| Poll interval | `SYNC_INTERVAL` | `--interval` |

### Example `.env` File

```dotenv
TODOIST_API_TOKEN=your-todoist-api-token
TODOIST_PROJECT=Work
JIRA_URL=https://yourcompany.atlassian.net
JIRA_EMAIL=you@company.com
JIRA_API_TOKEN=your-jira-api-token
JIRA_PROJECT=PROJ
SYNC_INTERVAL=5m
```

Environment variables set in the shell take precedence over `.env` values. The `.env` file is in `.gitignore` by default.

## Usage

### One-shot sync

```bash
todoist-jira-sync sync
```

### Watch mode (continuous polling)

```bash
todoist-jira-sync watch --interval 5m
```

Use `Ctrl+C` to stop watch mode gracefully.

### Environment-based usage

```bash
export TODOIST_API_TOKEN="..."
export TODOIST_PROJECT="Work"
export JIRA_URL="https://yourcompany.atlassian.net"
export JIRA_EMAIL="you@company.com"
export JIRA_API_TOKEN="..."
export JIRA_PROJECT="PROJ"

todoist-jira-sync sync
```

## How It Works

### State Tracking

Instead of maintaining a local database, cross-links are embedded in descriptions:

- Todoist tasks get a footer: `synced-jira-key: PROJ-123`
- Jira issues get a footer: `synced-todoist-id: 12345678`

### Sync Algorithm

1. Fetch all Todoist tasks in the configured project
2. Fetch all Jira issues in the configured project
3. Match linked pairs by parsing description footers
4. Create new items on either side for unlinked entries
5. For linked pairs, compare timestamps and sync the newer side to the older

### Comments

Comments are synced with attribution prefixes (`[From Todoist]` / `[From Jira]`) to prevent infinite sync loops and provide clear provenance.

## Development

```bash
make lint    # Run linters
make test    # Run tests
```
