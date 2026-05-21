# safe-mcp

**Give your AI agent a real shell. Keep it in a box.**

[![Docker](https://img.shields.io/badge/docker-ghcr.io%2Frthomazel%2Fsafe--mcp-blue?logo=docker)](https://ghcr.io/rthomazel/safe-mcp)
[![License: MIT](https://img.shields.io/badge/license-MIT-green)](./LICENSE)
[![Platforms](https://img.shields.io/badge/platforms-amd64%20%7C%20arm64-lightgrey)](#)

safe-mcp is an open-source MCP server that gives AI agents real shell access — isolated inside a Docker container. Your agent can read files, run commands, install dependencies, and work across multiple projects. Your host machine stays untouched.

No custom sandboxing layer. No trust required. Just Docker doing what Docker does.

---

## What your agent can do

| Tool | What it does |
| --- | --- |
| `context` | Discover the environment: OS, installed tools, mounted projects |
| `exec_sync` | Run a foreground command and get stdout/stderr back immediately |
| `exec_background` | Kick off a slow command without blocking |
| `status` | Poll a background job for results |
| `setup` | Install a project's language runtime and dependencies |

Agents can read and edit files, run tests, run linters, call CLIs, manage git — anything a developer can do in a terminal.

---

## Why safe-mcp

**Agents need a real environment.** Giving an agent only file-read tools means it can't run tests, can't verify its own changes, can't install a dependency. safe-mcp gives agents a full shell so they can actually finish the job.

**Container isolation is the right primitive.** Instead of a custom permission system, safe-mcp uses Docker volumes to define exactly what the agent can see and touch. Anything not mounted is invisible. Read-only mounts are supported. The container is ephemeral by default — nothing leaks between sessions.

**Works with the clients you already use.** stdio for Claude Desktop, HTTP/SSE for LibreChat, OpenAI-compatible HTTP for Open WebUI. One image, all transports.

---

## Quickstart

### 1. Pull the image

```bash
docker pull ghcr.io/rthomazel/safe-mcp:latest
# amd64 (most desktops/laptops) and arm64 (Apple Silicon, Raspberry Pi) builds available
```

### 2. Write a compose file

Two sample files are included depending on your transport:

| File | Transport | Works with |
| --- | --- | --- |
| `docker-compose-sample.yml` | stdio | Claude Desktop, CLI clients |
| `docker-compose-http-sample.yml` | HTTP/OpenAI | Open WebUI |
| `docker-compose-http-sample.yml` | HTTP/MCP-SSE | LibreChat, any SSE MCP client |

Copy a sample, edit the volume paths to point at your projects:

```bash
mkdir safeMCP && cd safeMCP
cp /path/to/docker-compose-sample.yml docker-compose.yml
${EDITOR:-vi} docker-compose.yml
```

Mount your projects under `/projects` (or anywhere — the agent discovers them dynamically via `context`).

For advanced volume tricks (read-only mounts, hiding subdirectories) see [doc/volume-mounting-tricks.md](./doc/volume-mounting-tricks.md).

### 3. Wire up your client

#### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS) or `~/.config/Claude/claude_desktop_config.json` (Linux):

```json
{
  "mcpServers": {
    "safe-mcp": {
      "command": "docker",
      "args": [
        "compose", "-f", "/path/to/safeMCP/docker-compose.yml",
        "run", "--rm", "-i", "safe-mcp"
      ]
    }
  }
}
```

Restart Claude Desktop.

> **Tip:** If tools disappear after an image update, rename the server key (e.g. `safe-mcp` → `1_safe-mcp`). This is a known Claude Desktop bug — renaming forces re-registration.

#### Open WebUI / HTTP clients

```bash
docker compose -f docker-compose-http-sample.yml up -d
```

Then add `http://localhost:8001` as an MCP server in your client. Set `SAFE_MCP_TRANSPORT` to `mcpo` for OpenAI-compatible REST or `mcp-proxy` for native MCP/SSE.

### 4. Install project dependencies

The container ships with `bash`, `python3`, and [mise](https://mise.jdx.dev) for language version management. Everything else is installed on demand.

If your project has a `.tool-versions` or `mise.toml`, mise installs the right runtime automatically. The `setup` tool then handles dependencies:

| File | Command |
| --- | --- |
| `go.mod` | `go mod download` |
| `yarn.lock` | `yarn install` |
| `package.json` | `npm install` |
| `requirements.txt` | `pip install -r requirements.txt` |
| `pyproject.toml` | `pip install .` |
| `Cargo.toml` | `cargo fetch` |
| `Gemfile` | `bundle install` |
| `mix.exs` | `mix deps.get` |

For custom bootstrapping, drop a `bin/setup` (or `setup.sh`) in your project root — `setup` will find and run it.

Two named volumes persist across sessions: `/mise` (language runtimes) and `/root` (home directory, binaries in `/root/bin`). Install ad-hoc tools to `/root/bin` to keep them between runs.

### 5. Write an agent prompt

The agent needs to know to call `context` at the start of a session. Here's a minimal system prompt:

````markdown
Call the safe-mcp `context` tool at the start of each session to orient yourself.

Use `exec_sync` for most file tasks (cat, find, grep). Use `exec_background` for slow commands and poll with `status`. You can do other work while waiting.

Editing files: use Python via `exec_sync` with a quoted heredoc to avoid shell interpolation issues.

```python
python3 << 'PYEOF'
with open('/projects/myproject/file.go', 'r') as f:
    content = f.read()
content = content.replace('old', 'new')
with open('/projects/myproject/file.go', 'w') as f:
    f.write(content)
print('ok')
PYEOF
```
````

---

## Persistence model

The container is ephemeral. Between sessions:

- **Survives:** anything on a named or bind-mounted volume
- **Lost:** anything installed to the container filesystem

| Path | Volume | Notes |
| --- | --- | --- |
| `/mise` | `safe-mcp-mise` | Language runtimes installed by mise |
| `/root` | `safe-mcp-root` | Home dir, `/root/bin` is on PATH |
| `/projects/*` | your bind mounts | Your actual project files |

---

## Logs

Logs are written in plain text to stderr.

---

## License

Check run script.
Comments `# -- ` above each `case` are used for help message.

## Optional: SSH key for git push from the container

By default the container has no SSH key, so `git push` will fail against SSH remotes.
If you want agents to be able to push to GitHub from inside the container, generate a dedicated key on your host and add it to your GitHub account.

```bash
# generate a key on your host
ssh-keygen -t ed25519 -f ~/.ssh/agents_id_ed25519 -N "" -C "jail-mcp container"

# print the public key — add it at github.com/settings/keys
cat ~/.ssh/agents_id_ed25519.pub
```

Then mount the private key into the container by adding this line to your `docker-compose.yml` volumes:

```yaml
# mount only the key file, not the .ssh directory — /root is a named volume and mounting the directory would shadow it entirely
- /home/you/.ssh/agents_id_ed25519:/root/.ssh/id_ed25519:ro
```

On first use, open a shell into the container and add GitHub to known hosts:

```bash
docker compose run --rm jail-mcp bash
# then inside the container
ssh-keyscan github.com >> ~/.ssh/known_hosts
```

This only needs to run once — `/root` is a named volume so `known_hosts` persists across sessions.
