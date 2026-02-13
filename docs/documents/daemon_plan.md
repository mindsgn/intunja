Daemon plan for running engine as a daemon

Goal
- Allow the CLI to start the engine (and HTTP server) as a background daemon so downloads continue after the CLI exits.

Design
1. Add daemon control verbs to the CLI: `daemon start`, `daemon stop`, `daemon status`, and an internal `daemon run`.
2. `daemon start` will spawn a detached child process running the same binary with `daemon run`, write the child's PID to a PID file (in the temporary directory), and exit.
3. `daemon run` will start the server (which initializes the engine) and run in foreground (this is the long-lived background process).
4. `daemon stop` will read PID file, send SIGTERM to the process, and remove the PID file.
5. `daemon status` will report whether a PID file exists and whether the process is alive.
6. Use a simple PID file at `/tmp/intunja-daemon.pid` for cross-platform simplicity.

Implementation steps
- Update `cmd.Run` to parse `os.Args` for `daemon` subcommands.
- Add helper functions to start/stop/status the daemon using `os/exec` and `syscall` to detach the child.
- On `daemon run`, use `server.Server.Run` so the daemon provides the HTTP API + engine use.
- Update `main.go` to pass the application version into `cmd.Run`.
- Write basic tests manually by starting the daemon, checking `status`, and stopping it.

Notes
- This uses a PID file and Unix signals; on Windows the behavior may be limited. For production cross-platform support consider using a native service manager (systemd/launchd/Windows service) or a process supervisor.
- PID file path can be changed later to a user config directory.
