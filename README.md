# jail-mcp

MCP server providing shell access to clients, jailed in a container.

> **Running outside Docker is not supported.** By default the server runs as root in a container.

The following tools are exposed to agents

| tool            | use case                          |
| --------------- | --------------------------------- |
| context         | project and environment discovery |
| exec sync       | run foreground commands           |
| exec background | run background jobs               |
| status          | poll job status                   |
| setup           | install project dependencies      |

## Configuration

### Overview

- 1 pull image
- 2 compose file with projects as volumes
- 3 configure clients
- 4 run setup to install project dependencies
- 5 add an agent prompt

### **1. Pull image**

```bash
docker pull ghcr.io/rthomazel/jail-mcp:latest
# builds available: AMD64 (most personal computers) and ARM64 (apple devices, niche hardware)
```

### **2. Compose file**

#### stdio

In this mode the MCP is spawned as a subprocess and communicates directly with the parent process (the agent application users interface with)

#### HTTP

Server mode, HTTP requests are used to communicate with the agent application (port 8001).
Server and application are separate processes.
There are two possible formats

- OpenAi: used with clients like Open WebUI.
- MCP/SSE: LibreChat, other apps that connect to MCP tools using SSE.

Two sample compose files are provided depending on your transport needs

| file                             | mode                   | use case                    |
| -------------------------------- | ---------------------- | --------------------------- |
| `docker-compose-sample.yml`      | stdio                  | Claude Desktop, CLI clients |
| `docker-compose-http-sample.yml` | HTTP/OpenAI-compatible | Open WebUI                  |
| `docker-compose-http-sample.yml` | HTTP/MCP-SSE           | LibreChat, HTTP MCP clients |

Copy the sample file, save and edit it.

```bash
mkdir jailMCP
cd jailMCP
${EDITOR-vi} docker-compose.yml
# paste contents
```

Update the volume paths to point to your real work.
The server discovers them dynamically, `/projects` is a suggestion.
Only paths bind-mounted as volumes can be modified in your machine, the MCP server is isolated in a container.
The container is ephemeral.
Only named volumes (`/mise`, `/root`) persist.
To install ad-hoc tools that survive across sessions, install to `$HOME/bin` (`/root/bin`), which is on the `jail-mcp-root` volume.
There are only two environment variables that can be used to set command timeouts and both example files have default values. 

For tricks on how to mount paths read-only or hide sub-directories see [volume-mounting-tricks.md](./doc/volume-mounting-tricks.md)

### **3. Wire up clients**

See instructions in your client application how to add an MCP tool.

### Example: Claude Desktop (stdio)

Spawns a fresh container per session via `docker compose run`.
`--rm` removes it after each session, only persistent paths survive.

_MacOS:_
Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

_Linux:_ Add to `~/.config/Claude/claude_desktop_config.json`

```json
{
  "mcpServers": {
    "jail-mcp": {
      // Linux: just "docker"
      "command": "/Applications/Docker.app/Contents/Resources/bin/docker",
      "args": [
        "compose",
        "-f",
        // Linux: /home/you
        "/Users/you/Desktop/jailMCP/docker-compose.yml",
        "run",
        "--rm",
        "-i",
        "jail-mcp"
      ]
    }
  }
}
```

Restart claude desktop.

### Example: Open WebUI / HTTP clients

Runs a persistent container exposing an HTTP MCP endpoint on port 8001.

```bash
docker compose -f docker-compose-http-sample.yml up -d
```

Then add `http://localhost:8001` as an MCP tool in your client.

The HTTP transport is configured via `JAIL_MCP_TRANSPORT` in the container environment — `mcpo` for OpenAI-compatible REST (Open WebUI) or `mcp-proxy` for native MCP/SSE (LibreChat). See `docker-compose-http-sample.yml` for an example.

#### Known Claude Desktop Bugs

When updating the MCP server to a new build, Claude desktop may show errors or fail to discover tools.
This has been observed to happen when changing permission settings as well.
This can be fixed by renaming the server in the configuration above (e.g. `jail-mcp` → `1_jail-mcp`), which forces the client to treat it as a new server and re-register the tools.
Renaming the first letter seems to be important.

### **4. Setup**

Because the container is an isolated environment, the programming language and project dependencies have to be installed.

#### programming language installation

The container has only `bash` and `python3`, for basic scripting, programming languages are not included by design.
[Mise](https://mise.jdx.dev) is installed for language version management.
It's expected that the language will be versioned using a `.tool-versions` file or `mise.toml` for each project.
Once this file is present the setup tool will call mise to install the language.

#### project dependencies

The setup tool also recognizes popular programming languages dependency files and installs them.

| file               | setup command                        | reference                    |
| ------------------ | ------------------------------------ | ---------------------------- |
| ".tool-versions"   | "mise install"                       | programming languages & clis |
| "go.mod"           | "go mod download && go install tool" | Go                           |
| "yarn.lock"        | "yarn install"                       | JavaScript                   |
| "package.json"     | "npm install"                        | JavaScript                   |
| "requirements.txt" | "pip install -r requirements.txt"    | Python                       |
| "pyproject.toml"   | "pip install ."                      | Python                       |
| "Gemfile"          | "bundle install"                     | Ruby                         |
| "Cargo.toml"       | "cargo fetch"                        | Rust                         |
| "mix.exs"          | "mix deps.get"                       | Erlang/Elixir                |

For further project bootstraping, the setup tool will look for a `setup.sh` bash script and execute it.
There are a few locations we expect to find this file besides the project root.

    # possible locations of the setup script
    "setup.sh",
    "setup",
    "bin/setup",
    "script/setup",
    "scripts/setup",
    "scripts/setup.sh",

### **5. Agent prompt**

To discover projects, the agent needs to call the context tool.
The system prompt should make this requirement clear and also explain how to how use exec sync and exec background.
Have the agent run the setup tool in the project directory at the start of the session to install dependencies.

```markdown
# sample prompt

Call the jail MCP context tool at the start of each session to orient yourself.
Use exec_sync for most file tasks (cat, find, grep, sed). This is the only way to interact with project files.
Use exec_background for slow commands; poll with the status tool. You can do other work while waiting.
If the project's language isn't installed, run the setup tool on the project path first.

Editing files via jail:

- Use Python via exec_sync.
- Always use a quoted heredoc (<< 'PYEOF') to prevent bash from interpreting backticks, $variables, or special characters inside the Python code.
- Prefer two small targeted replaces over one large multi-line block match — large blocks are brittle.

python3 << 'PYEOF'
with open('/projects/server/path/to/file', 'r') as f:
    content = f.read()
content = content.replace('old', 'new')
with open('/projects/server/path/to/file', 'w') as f:
    f.write(content)
print('ok')
PYEOF
```

## Logs

Logs are written in plain text to stderr.

## Dev

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
