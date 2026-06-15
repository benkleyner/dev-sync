# dev-sync

A small CLI that watches local directories and syncs changes to remote SFTP destinations. Configure once, run as a background daemon, and your remote tree stays in step with your local edits.

One-way only (local → remote). Multiple sync pairs run concurrently in a single process.

## Features

- Interactive onboarding (`dev-sync init`) — prompts for connection details and verifies the remote directory exists before saving
- Verifies SFTP host keys using `~/.ssh/known_hosts`; `init` can prompt to trust an unknown host
- Multiple sync pairs run concurrently from one daemon
- Initial sync on startup, then real-time updates via filesystem events
- Debounced reconciliation — editor save-storms collapse into one accurate log line per change
- Respects `.gitignore`; skips `.git/` directories
- Structured JSON logging to a file; `dev-sync logs` for a pretty, recent view
- Background daemon with proper start/stop/status lifecycle

## Install

```sh
brew install --cask bkleyner/tap/dev-sync
```

Release tags publish darwin/linux binaries and update the Homebrew tap via GoReleaser.

## Build

Requires Go 1.25 or newer.

```sh
go build -o dev-sync .
```

Or run without building: `go run . <command>`.

## Quick start

```sh
# 1. configure your first sync pair (will verify the SFTP connection)
dev-sync init

# 2. add more pairs if you want
dev-sync init

# 3. see what's configured
dev-sync list

# 4. start the background daemon
dev-sync start

# 5. check on it
dev-sync status
dev-sync logs

# 6. stop when you're done
dev-sync stop
```

## Commands

| Command | What it does |
| --- | --- |
| `init` | Interactive setup for a new sync pair. Verifies SFTP creds and remote dir. |
| `list` | Show configured sync pairs. |
| `run` | Run all pairs in the foreground (logs to terminal, Ctrl-C to stop). |
| `start` | Launch the daemon in the background. |
| `stop` | Stop the running daemon. |
| `status` | Report whether the daemon is running. |
| `logs [n]` | Print the last `n` log entries (default 50). |
| `scan <dir>` | List every file under `<dir>`, respecting `.gitignore`. Debug helper. |
| `mirror <src> <user@host:dir>` | One-off SFTP mirror without using config. Requires `DEV_SYNC_PASSWORD` env var. |
| `version` | Print the version. |

`run` and `start` execute the same syncing logic. `run` is for interactive development (logs in your terminal); `start` is for set-and-forget background operation.

## Host key verification

`dev-sync` uses the standard OpenSSH `~/.ssh/known_hosts` file. During `dev-sync init`, an unknown SFTP host key is shown with its key type and SHA256 fingerprint, and you can choose whether to add it to `known_hosts`. Runtime commands (`run`, `start`, and `mirror`) do not prompt; unknown, changed, or revoked host keys fail closed.

## Configuration

Config, pidfile, and log file live under your platform's user config directory:

- macOS: `~/Library/Application Support/dev-sync/`
- Linux: `~/.config/dev-sync/`

Contents:

- `config.json` — sync pairs and keychain password references (created by `init`)
- `daemon.pid` — written on `start`, removed on `stop`
- `daemon.log` — JSON-lines log written by the background daemon

The config file is created with `0600` permissions; the directory with `0700`.

SFTP passwords are stored in the OS keychain via `go-keyring`, and legacy plaintext passwords are migrated the next time `init`, `list`, `run`, or `start` loads the config.

## Known limitations

- **No retry on transient SFTP failures.** A network blip currently stops that pair until the daemon is restarted. Auto-reconnect with exponential backoff is planned.
- **One-way sync only.** Remote-side changes are not detected and may be overwritten on the next local change.
- **No nested `.gitignore` support.** Only the root `.gitignore` is consulted.
- **Background daemon control is Unix-only.** `start`/`stop`/`status` are disabled on Windows.
