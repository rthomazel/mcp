# bench-mcp

**Give your AI agent a real workbench.**

[![Docker](https://img.shields.io/badge/docker-ghcr.io%2Frthomazel%2Fbench--mcp-blue?logo=docker)](https://ghcr.io/rthomazel/mcp/bench)
[![License: BSD3](https://img.shields.io/badge/license-BSD3-green)](../LICENSE)
[![Platforms](https://img.shields.io/badge/platforms-amd64%20%7C%20arm64-lightgrey)](#)

bench-mcp is an open-source MCP server that gives AI agents the fundamental tools to get real work done: a real shell, background job control, precise file editing, and environment discovery — all isolated inside a Docker container.
Your host machine stays untouched.
No custom sandboxing layer. No trust required. Just Docker doing what Docker does.

---

## What your agent can do

| Tool               | What it does                                                             |
| ------------------ | ------------------------------------------------------------------------ |
| `context`          | Discover the environment: OS, installed tools, mounted projects          |
| `shell`            | Run a foreground command and get stdout/stderr back immediately          |
| `shell_background` | Kick off a slow command without blocking                                 |
| `status`           | Poll a background job for results                                        |
| `setup`            | Install a project\'s language runtime and dependencies                   |
| `file_replace`     | Find and replace unique substrings in a file. Returns a unified diff     |
| `file_replace_all` | Replace all occurrences of a substring in a file. Returns a unified diff |

Agents can read and edit files, run tests, run linters, call CLIs, manage git — anything a developer can do in a terminal.

---

## Why bench-mcp

**Agents need a real environment.** Giving an agent only file-read tools means it can\'t run tests, can\'t verify its own changes, can\'t install a dependency. bench-mcp gives agents a full shell so they can actually finish the job.

**Container isolation is the right primitive.** Instead of a custom permission system, bench-mcp uses Docker volumes to define exactly what the agent can see and touch. Anything not mounted is invisible. Read-only mounts are supported. The container is ephemeral by default — nothing leaks between sessions.

**Works with the clients you already use.** stdio for Claude Desktop, HTTP/SSE for LibreChat, OpenAI-compatible HTTP for Open WebUI. One image, all transports.

---

## Quickstart

### 1. Pull the image

```bash
docker pull ghcr.io/rthomazel/mcp/bench:latest
# amd64 (most desktops/laptops) and arm64 (Apple Silicon, Raspberry Pi) builds available
```

### 2. Write a compose file

#### stdio

In this mode the MCP is spawned as a subprocess and communicates directly with the parent process (the agent application users interface with)

#### HTTP

Server mode, HTTP requests are used to communicate with the agent application (port 8001).
Server and application are separate processes.
There are two possible formats

- OpenAi: used with clients like Open WebUI.
- MCP/SSE: LibreChat, other apps that connect to MCP tools using SSE.

Two sample compose files are provided depending on your transport needs

#### Choosing one

Two sample files are included depending on your transport.
HTTP SSE is recommended.

| File                             | Transport    | Works with                    |
| -------------------------------- | ------------ | ----------------------------- |
| `docker-compose-sample.yml`      | stdio        | Claude Desktop, CLI clients   |
| `docker-compose-http-sample.yml` | HTTP/OpenAI  | Open WebUI                    |
| `docker-compose-http-sample.yml` | HTTP/MCP-SSE | LibreChat, any SSE MCP client |

Copy a sample, edit the volume paths to point at your projects:

```bash
mkdir benchMCP && cd benchMCP
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
    "bench-mcp": {
      "command": "docker",
      "args": [
        "compose",
        "-f",
        "/path/to/benchMCP/docker-compose.yml",
        "run",
        "--rm",
        "-i",
        "bench-mcp"
      ]
    }
  }
}
```

Restart Claude Desktop.

> **Tip:** If tools disappear after an image update, rename the server key (e.g. `bench-mcp` → `1_bench-mcp`). This is a known Claude Desktop bug — renaming forces re-registration.

#### Open WebUI / HTTP clients

```bash
docker compose -f docker-compose-http-sample.yml up -d
```

Then add `http://localhost:8001` as an MCP server in your client. Set `BENCH_MCP_TRANSPORT` to `mcpo` for OpenAI-compatible REST or `mcp-proxy` for native MCP/SSE.

### 4. Setup project in container

The container ships by design only with `bash`, `python3`, and [mise](https://mise.jdx.dev) for language version management.
Remember that the bench is isolated from the host machine, so the binaries in the host system won't be available.

#### Setup tool & mise

The setup tool bootstraps the project, your prompt should ask the agent to run it before starting work.
For programming languages and other tools create a mise file in the project root.
It will become the source for versioning your tooling.

```toml
golang 1.26.4
oxfmt latest
gh latest
```

Mise installs the right runtime automatically during the setup process.
Check the [mise](https://mise.jdx.dev) documentation on how to get started.

#### Dependencies and libraries

The `setup` tool then installs dependencies:

| File               | Command                           |
| ------------------ | --------------------------------- |
| `go.mod`           | `go mod download`                 |
| `yarn.lock`        | `yarn install`                    |
| `package.json`     | `npm install`                     |
| `requirements.txt` | `pip install -r requirements.txt` |
| `pyproject.toml`   | `pip install .`                   |
| `Cargo.toml`       | `cargo fetch`                     |
| `Gemfile`          | `bundle install`                  |
| `mix.exs`          | `mix deps.get`                    |

#### Customization

If your project requires further setup, write your own script.
Drop a bash `bin/setup` (or `setup.sh`) in your project root — `setup` will find and run it.

    # possible locations of the setup script
    "setup.sh",
    "setup",
    "bin/setup",
    "script/setup",
    "scripts/setup",
    "scripts/setup.sh",

## Persistence model

Two named volumes persist across sessions: `/mise` (language runtimes) and `/root` (home directory, binaries in `/root/bin`).
Install ad-hoc tools to `/root/bin` to keep them between runs.
The container is ephemeral. Between sessions:

- **Survives:** anything on a named or bind-mounted volume
- **Lost:** anything installed to the container filesystem

| Path          | Volume           | Notes                               |
| ------------- | ---------------- | ----------------------------------- |
| `/mise`       | `bench-mcp-mise` | Language runtimes installed by mise |
| `/root`       | `bench-mcp-root` | Home dir, `/root/bin` is on PATH    |
| `/projects/*` | your bind mounts | Your actual project files           |

---

### 5. Write an agent prompt

You need to provide guidance on how to use the tools.
Here\'s a minimal system prompt:

```markdown
Call the bench-mcp `context` tool at the start of each session to orient yourself.
Then run the 'setup' tool to install project dependencies.

Use `shell` for most file tasks (cat, find, grep).
Use `shell_background` for slow commands and poll with `status`.
You can do other work while waiting.

Editing files:

- Use `file_replace` for targeted edits — finds a unique substring and replaces it. Returns a unified diff.
- Use `file_replace_all` to replace every occurrence of a substring (e.g. renaming a symbol).
```

---

## 6. SSH key for git push from the container (optional)

By default the container has no SSH key, so `git push` will fail against SSH remotes.
If you want agents to be able to push to GitHub from inside the container, generate a dedicated key on your host and add it to your GitHub account.

```bash
# generate a key on your host
ssh-keygen -t ed25519 -f ~/.ssh/agents_id_ed25519 -N "" -C "bench-mcp container"

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
docker compose run --rm bench-mcp bash
# then inside the container
ssh-keyscan github.com >> ~/.ssh/known_hosts
```

This only needs to run once — `/root` is a named volume so `known_hosts` persists across sessions.

---

## Logs

Logs are written in plain text to stderr.

---

## License

BSD 3-Clause License
