# todoist-jira-sync

A simple CLI tool for bidirectional synchronization between Todoist and Jira Cloud.

## Run

```sh
go run . -h    # Help and config
go run . sync  # Manual sync
go run . watch # Sync periodically
```

## Install

On Mac, run `./install.sh` to install the app as a LaunchAgent that runs every 5 minutes.
