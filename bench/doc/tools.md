# tools

Dev tools (languages, formatters, linters, etc.) should be versioned alongside the project that needs them, not installed globally in the container.

## mise

Tools are managed per-project via mise using a `.tool-versions` file.
When `setup` runs on a project, mise installs the declared tool versions into `/mise/installs/` and creates shims in `/mise/shims/`.
The server prepends `/mise/shims` to `PATH` at startup so all subprocesses can use them.
The `context` tool lists available shims under `mise shims:` by reading `/mise/shims` directly — no hardcoding.

### adding a language or tool to a project

1. Add the entry and version to the project's `.tool-versions`
2. Call `setup` on the project path — mise will install it
3. The shim appears in `/mise/shims/` and is immediately available to `shell`

## how setup works

`setup` accepts one or more project paths and launches a background job per path in parallel.
Each path is inspected independently using two mechanisms: manifest rules and a setup script.

### manifest rules

`setup` walks a fixed ordered list of manifest files, every match appends its command to the job.
The full list in order:

| manifest file      | command                              |
| ------------------ | ------------------------------------ |
| `.tool-versions`   | `mise install`                       |
| `go.mod`           | `go mod download && go install tool` |
| `yarn.lock`        | `yarn install`                       |
| `package.json`     | `npm install`                        |
| `requirements.txt` | `pip install -r requirements.txt`    |
| `pyproject.toml`   | `pip install .`                      |
| `Gemfile`          | `bundle install`                     |
| `Cargo.toml`       | `cargo fetch`                        |
| `mix.exs`          | `mix deps.get`                       |

Multiple manifests can match — for example a Go project with `.tool-versions` produces `mise install && go mod download && go install tool`.

### setup script

In parallel with manifest detection, `setup` looks for a setup script by checking these candidates in order, taking the first regular file found:

`setup.sh`, `setup`, `bin/setup`, `script/setup`, `scripts/setup`, `scripts/setup.sh`

### combining both

The command runs as a background job in the project directory.
Poll with `status` using the returned `job_id`.

## background jobs and resource management

`shell_background` jobs are deprioritized: they spawn as `nice -n <level> bash -c <cmd>` (level from `BENCH_MCP_BACKGROUND_NICE`, default 10), so their whole process tree runs at a lower CPU scheduling priority. A heavy background job therefore cannot starve foreground `shell` commands, which keep their default priority and preempt the deprioritized tree.

**Known gap (documented, not enforced):** nice changes only CPU scheduling priority. It does **not** bound memory (a very large background job can still exhaust container memory), and it does **not** cap the *aggregate* CPU of several simultaneous background jobs — two or more heavy jobs can still collectively delay a new `shell` command's startup on a small box. `setup` jobs run at default priority (not deprioritized) by design. Hard isolation (per-job cgroup memory/CPU caps) is out of scope; see `doc/resource-management-design.md` for the future direction.

## dependencies

Each project owns its tool versions via its package manager manifest.

- JavaScript: `package.json` devDependencies installed by `yarn install` or `npm install`
- Python: `requirements.txt` or `pyproject.toml` installed by `pip install`
- etc.
