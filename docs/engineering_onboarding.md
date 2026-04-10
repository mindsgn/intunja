Engineering Onboarding (Junior-friendly)

Purpose
- Give a new engineer a clear, short path to get productive with the codebase.

Prerequisites
- Go 1.21+ installed
- Basic familiarity with Go modules and `go build`/`go test`
- Optional: `make` for convenience scripts

Getting the repo
1. Clone:
   git clone <repo>
   cd intunja
2. Fetch deps:
   go mod download

Quick dev cycle (TUI)
1. Build:
   cd core
   go build -o intunja .
2. Run in foreground TUI:
   ./intunja
3. Run in headless (CI-friendly):
   ./intunja headless
4. Daemon:
   ./intunja daemon start|stop|status

Project layout (high level)
- `core/cmd/cli.go` — TUI entrypoint and Bubble Tea model
- `core/engine/` — Engine facade that wraps `anacrolix/torrent`
  - `engine.go` — core lifecycle and API used by app
  - `torrent.go` — per-torrent state wrapper
  - `remote.go` — HTTP proxy to remote daemon
  - `interface.go` — shared engine interface
- `core/server/` — HTTP server and API handlers
- `docs/` — project documentation and plans

How to make a small code change
1. Create a branch: `git checkout -b fix/your-change`
2. Edit files under `core/`
3. Run `go build ./...` to ensure compilation
4. Add unit tests in `*_test.go` and run `go test ./...`
5. Commit and push, open PR

Testing
- Unit tests: `go test ./...`
- For TUI behavior, use `headless` mode or run interactive tests manually.

Where to find more
- `docs/documentation_audit.md` — what docs to prioritize
- `docs/mobile_roadmap.md` — mobile integration plan
- `docs/notes.md` — engineering notes and design decisions

Common gotchas
- The engine re-creates the underlying torrent client on `Configure()`. Reconfiguration will drop current torrents.
- If a daemon is running, the TUI uses a remote proxy instead of creating a local client.

Contact
- For questions, ping the maintainer or add an issue with the `help` tag.
