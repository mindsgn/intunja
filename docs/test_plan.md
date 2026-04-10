Test plan — unit, integration, and e2e

Goals
- Provide reliable unit test coverage for engine logic, parsing, and DB layer.
- Provide integration tests that exercise adding magnets and basic torrent lifecycle against a mocked anacrolix client.
- Provide end-to-end tests for CLI headless mode and daemon interactions.

Test types
1. Unit tests
   - Target: `engine` package functions: `percent()`, `upsertTorrent()`, `StartTorrent/StopTorrent` logic (use small in-memory structs or interfaces).
   - Parsing tests: magnet link parsing, tracker URL normalization.
   - DB tests (when implemented): schema creation, CRUD operations (use temp file DB `:memory:` or tmp file).

2. Integration tests
   - Mock the `torrent.Client` behavior (use an interface or small test double) to simulate `GotInfo()` and piece progress.
   - Test adding a magnet, waiting for metadata, and starting download flow using channels.

3. End-to-end tests
   - Headless mode: start the app in headless mode in a subprocess and assert expected output lines (use `os/exec` to capture stdout).
   - Daemon interaction: start `./intunja daemon start` in a temp workspace, run headless TUI that detects daemon, and assert no bind error.

Testing strategy
- Keep unit tests fast and isolated.
- Use `go test -race ./...` in CI to detect data races.
- For integration tests that require network, stub out actual network calls or run within CI with controlled environment.

CI
- Add a GitHub Actions workflow:
  - Steps: checkout, set up Go, run `go test ./...` with `-race`, build artifacts, optional static checks (golangci-lint).

First tasks to implement
1. Add `engine` package unit tests for `percent()` and `updateLoaded()` with a small fake `torrent.Torrent` object.
2. Add parsing tests for magnet/tracker normalization.
3. Add a simple headless e2e test that runs `./intunja headless` for 2 seconds and confirms it prints at least one line.

Notes
- Some tests require isolating `anacrolix/torrent`; introduce small adapters or use `gomock` to mock behavior if needed.
