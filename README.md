# streamsh

Give coding agents eyes on your terminal. `streamsh` wraps your shell session and streams output to a daemon that agents query via MCP.

Agents can search, page through, and (optionally) write to your terminal -- no copy-pasting required.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/arnavsurve/streamsh/main/install.sh | sh
```

Installs prebuilt binaries to `~/.local/bin`. Override with `INSTALL_DIR`:

```sh
curl -fsSL https://raw.githubusercontent.com/arnavsurve/streamsh/main/install.sh | INSTALL_DIR=/usr/local/bin sh
```

<details>
<summary>Other install methods</summary>

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

</details>

## Setup

Register `streamshd` as an MCP server so your agent can access terminal sessions.

**Claude Code:**

```sh
claude mcp add -s user streamsh -- streamshd
```

<details>
<summary>Manual config</summary>

Add to `~/.claude.json`:

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

</details>

**OpenCode:**

Add to `opencode.json` (project root) or `~/.config/opencode/opencode.json` (global):

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

Start a tracked shell session:

```sh
streamsh
```

That's it. Your agent can now see everything in your terminal.

### Options

```
--title "name"    Label the session (default: auto-generated)
--collab          Allow the agent to send input to your terminal
--shell /bin/zsh  Override the default shell
```

### Collaborative mode

With `--collab`, agents can type into your session. This lets them run commands, respond to prompts, and interact with your shell directly:

```sh
streamsh --collab
```

The agent gets access to the `write_session` MCP tool, which sends raw text to your terminal's PTY. You'll see everything the agent types in real time.

## How it works

```
┌──────────┐       Unix socket       ┌───────────┐       MCP (stdio)       ┌───────────┐
│ streamsh │ ───────────────────────> │ streamshd │ <───────────────────── │   Agent   │
│ (client) │  output, commands, PTY  │  (daemon)  │  list, query, write   │           │
└──────────┘                         └───────────┘                         └───────────┘
```

- **streamsh** launches your shell inside a PTY, captures all output (with ANSI codes stripped), and forwards it to the daemon over a Unix socket.
- **streamshd** stores output in a per-session ring buffer (default 10,000 lines) and serves it to agents via three MCP tools:

| Tool | Description |
|------|-------------|
| `list_sessions` | List all active and recent sessions |
| `query_session` | Read output: last N lines, cursor pagination, or fuzzy search |
| `write_session` | Send input to a `--collab` session |

Sessions are identified by a short ID (e.g. `a1b2c3d4`) or by title.

## Daemon flags

`streamshd` is started automatically by your agent's MCP runtime. If needed, these flags are available:

```
--buffer-size 10000   Lines per session ring buffer
--log-level info      Log level: debug, info, warn, error
--socket <path>       Override Unix socket path
```
