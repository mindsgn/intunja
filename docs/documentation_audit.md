Documentation audit — what we need and what to remove

Date: 2026-02-15

1) Required documentation (must-have)
- Quickstart / Local dev: how to build, run, common flags, config file.
- Developer onboarding: project layout, where major components live (`cmd/`, `engine/`, `server/`), how data flows.
- Coding conventions & style: gofmt/govet expectations, testing commands.
- Contribution guide: branching, commit message style, PR process.
- Architecture overview: engine responsibilities, TUI vs server vs remote engine.
- Daemon usage: start/stop/status, PID file behavior.
- Headless mode & CI usage: how to run non-interactive tests.
- Persistence plan: where state is stored, schema notes (when implemented).
- Mobile roadmap summary (high-level decisions and constraints).

2) Nice-to-have documentation
- Detailed API spec for server endpoints (endpoints, payloads, examples).
- Troubleshooting guide with common errors and remedies.
- Design notes for subsystems (tracker handling, piece selection).
- Release notes and changelog.

3) Documents to remove or merge (keep repository focused)
- Small throwaway notes or duplicated content across multiple files.
- Long personal logs that are not actionable (move to archive if needed).
- Outdated migration drafts that aren't used.

4) Recommendations
- Consolidate onboarding material into a single `docs/engineering_onboarding.md` (see created file).
- Move tactical/working notes (experiment logs) into a `docs/experiments/` folder or archive.
- Keep `README.md` high-level with links to the onboarding, quickstart, and mobile roadmap.
