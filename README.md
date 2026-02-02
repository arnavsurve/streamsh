# streamsh

Lets coding agents read your shell output. Run `streamsh` to wrap your terminal session. Agents can then search and page through the output via MCP tools (`list_sessions`, `query_session`).

## Install

```sh
go install github.com/arnavsurve/streamsh/cmd/streamsh@latest
go install github.com/arnavsurve/streamsh/cmd/streamshd@latest
```

## Usage

Start the daemon (typically configured as an MCP server in your agent), then open a tracked session in your terminal:

```sh
streamsh
```

Each session is identified by a short UUID prefix (e.g. `a1b2c3d4`). You can optionally pass `--title "dev server"` for a human-friendly label.

Agents connect to `streamshd` as an MCP server (stdio) and can list sessions, read the last N lines, search output, or paginate with a cursor.
