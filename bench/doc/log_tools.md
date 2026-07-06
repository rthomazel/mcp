# MCP Log Tools — v1 Design Spec (final)

## Tools: `logs` and `logsRaw`

---

## `logs` — structured diagnostics, parser-first with raw fallback

```go
func logs(severity, pattern string) []Diagnostic
```

**MCP usage description:**

> Get build/lint/test diagnostics as structured `file:line [severity] message` entries. Filter by `severity` (e.g. "error", "warn") and/or `pattern` (regex over each rendered diagnostic). Falls back to raw numbered output automatically if structured diagnostics cannot be produced—you don't need to call `logsRaw` first. Each entry is tagged with its original raw log line number so you can pull surrounding context via `logsRaw` if needed.

**Params:**

* `severity` — exact match (structured diagnostics only; no-op for raw fallback)
* `pattern` — regex applied to the rendered diagnostic line

No `limit` param — output capped purely by char budget (`MaxResponseChars`).

**Output:**

```text
[044] internal/auth/session.go:44:2 [error] undefined: jwt.Parse
[044] internal/auth/session.go:44:15 [error] missing return statement
[012] src/components/Login.tsx:12:3 [warn] 'unused' is assigned a value but never used (no-unused-vars)

--- 3 diagnostics, 3 shown, not truncated ---
```

or when clamped:

```text
--- 20 diagnostics, 10 shown, truncated to 8000 chars ---
```

**Behavior:**

* Parser-first.
* Automatically falls back to raw numbered log lines if structured diagnostics cannot be produced.
* Fallback is transparent to the client.
* Diagnostics preserve original log order.
* Diagnostics originating from the same raw log line remain adjacent.
* Truncation never splits a diagnostic. If the next diagnostic would exceed the character budget, it is omitted entirely.

---

## `logsRaw` — raw log access

```go
func logsRaw(start int, end *int, pattern *string) LogResult
```

**MCP usage description:**

> Read raw log output by line range or regex search. `start=0` reads from the top; negative `start` (e.g. `-50`) reads the last N lines (tail); `start`+`end` reads an explicit range. `pattern` regex-searches the log, optionally scoped to the given range. Regex searches automatically include a small amount of surrounding context. Use this when `logs` doesn't have what you need—for example, viewing context around a diagnostic's line number or inspecting output from a tool without structured diagnostics.

**Semantics:**

* `start=0, end=nil` → whole file, char-budget clamped
* `start<0, end=nil` → tail, last `|start|` lines
* `start, end` set → explicit range
* `pattern` set → regex search, scoped to range if given; otherwise whole file
* Search results automatically include ±3 lines of surrounding context.

**Output:**

```text
[142] building main.go...
[143] internal/auth/session.go:44:2: undefined: jwt.Parse
[144] FAIL: 3 packages failed to build

--- 3 lines, 3 shown, not truncated ---
```

or when clamped:

```text
--- 4200 lines, 62 shown, truncated to 8000 chars ---
```

Non-contiguous search results are separated by a blank line:

```text
[44] internal/auth/session.go:44:2: undefined: jwt.Parse
[45] internal/auth/session.go:45:1: }
[46] exit status 1


[812] internal/auth/token_test.go:12:5: undefined: jwt.Parse
[813] internal/auth/token_test.go:13:1: }
[814] FAIL

--- 2 matches, 2 shown, not truncated ---
```

**Behavior:**

* Preserves original log order.
* Truncation never splits a line or a contiguous context block. If the next block would exceed the character budget, it is omitted entirely.

---

## Shared conventions

* **Numbered lines** `[NNN]` — raw log line numbers shared between both tools.
* **Stable references** — line numbers are portable between `logs` and `logsRaw`.
* **Original log order** is always preserved.
* **Char-budget clamp only** (`MaxResponseChars`) — no line/item limits and no `limit` parameters.
* **Regex everywhere** — `logs.pattern` and `logsRaw.pattern` use the same filtering model.
* **Transparent fallback** — clients never need to know whether `logs` returned structured diagnostics or raw fallback.
* **Deterministic output** — identical input logs produce identical ordering and line numbering.
* **Footer format:** `--- X total, Y shown, {not truncated | truncated to N chars} ---`. Total/shown counts are always present, and truncation status is always explicit.
