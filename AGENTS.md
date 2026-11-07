# Repository Guidelines & Command Execution

## Dev Container Execution

This repository relies on a Dev Container for all development toolchains (Go, CLI utilities, etc.). Do not assume development tools are installed on the host.

- When executing terminal commands (such as builds, tests, linting, database migrations, or running scripts), verify whether the agent environment has the necessary development tools.
- If operating from an environment without the necessary development tools, always prefix commands with:

  ```bash
  devcontainer exec --workspace-folder . <COMMAND>
  ```

  or run multi-command pipelines inside `devcontainer exec --workspace-folder . sh -c '<COMMANDS>'`.
- If the dev container is not already running, it can be started with:

  ```bash
  devcontainer up --workspace-folder .
  ```

- Direct file reads, file edits, and git status checks may still be performed against the workspace files directly on disk.

## CI & Pre-Flight Verification

GitHub Actions CI enforces that all linting, tests, and builds succeed. Before concluding changes, verify:

1. `go vet ./...`
2. `go test ./...`
3. `go build ./...`

## Repository Skills

Consult specialized skills located in `.agents/skills/`:

- None yet
