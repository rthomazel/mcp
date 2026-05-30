# volume mounting tricks

## Read-only paths

The example configuration shows how to add read-only paths, i.e `.git`.

```yaml
# :ro adds a path as read-only, must come after the parent path
- /Users/you/helloworld/.git:/projects/helloworld/.git:ro
```

## Hidden mounts

Sensitive files or directories inside a mounted project can be hidden from the agent using Docker volume mounts — no server changes needed.

Docker applies mounts in declaration order. A second mount over a subpath of an already-mounted project shadows it before the container process starts. The container has no `CAP_SYS_ADMIN` so runtime mounts are not possible; this must be done in the compose file.

**Hide a file** — mount `/dev/null` over it:

```yaml
volumes:
  - /Users/you/myproject:/projects/myproject
  - /dev/null:/projects/myproject/.env
```

**Hide a directory** — mount an empty host directory over it:

```yaml
volumes:
  - /Users/you/myproject:/projects/myproject
  - /tmp/bench-hidden:/projects/myproject/secrets
```

The empty dir must exist on the host (`mkdir -p /tmp/bench-hidden`). Mount order matters — the hide entry must come after the parent project mount, same rule as `:ro` overlays.
