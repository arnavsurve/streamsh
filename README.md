# streamsh

Lets coding agents read your shell output. Run `streamsh` to wrap your terminal session. Agents can then search and page through the output via MCP tools (`list_sessions`, `query_session`).

## Install

**Quick install:**

```sh
curl -fsSL https://raw.githubusercontent.com/arnavsurve/streamsh/main/install.sh | sh
```

This downloads prebuilt binaries to `~/.local/bin`. Set a custom location with `INSTALL_DIR`:

```sh
curl -fsSL https://raw.githubusercontent.com/arnavsurve/streamsh/main/install.sh | INSTALL_DIR=/usr/local/bin sh
```

**With Go:**

```sh
go install github.com/arnavsurve/streamsh/cmd/streamsh@latest
go install github.com/arnavsurve/streamsh/cmd/streamshd@latest
```

**From source:**

```sh
git clone https://github.com/arnavsurve/streamsh.git
cd streamsh
./install.sh
```

## MCP Setup

Register `streamshd` as an MCP server so your coding agent can query terminal sessions.

### Claude Code

```sh
claude mcp add -s user streamsh -- streamshd
```

This registers the server globally (user scope), making it available across all projects. Alternatively, add it manually to `~/.claude.json`:

```json
{
  "mcpServers": {
    "streamsh": {
      "type": "stdio",
      "command": "streamshd"
    }
  }
}
```

### OpenCode

Add to `opencode.json` in your project root, or `~/.config/opencode/opencode.json` for global access:

```json
{
  "mcp": {
    "streamsh": {
      "type": "local",
      "command": ["streamshd"]
    }
  }
}
```

## Usage

Start the daemon (typically configured as an MCP server in your agent), then open a tracked session in your terminal:

```sh
streamsh
```

Each session is identified by a short UUID prefix (e.g. `a1b2c3d4`). You can optionally pass `--title "dev server"` for a human-friendly label.

Agents connect to `streamshd` as an MCP server (stdio) and can list sessions, read the last N lines, search output, or paginate with a cursor. No need to run it manually -- this is the binary ran by your coding agent to host the MCP server.
