# streamsh

`streamsh` wraps your shell sessions and streams output to local agents via MCP.

Agents can search, page through, and (optionally) write to your terminal.

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

