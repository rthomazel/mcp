# keys — test plan

Unit tests live alongside source files (`_test.go`). Tests use `httptest.NewServer`
where an HTTP server is needed — no mocks, no external network calls.

## `internal/config` — `config_test.go`

Highest value: the validation logic is the first security gate. Cover all rejection
paths so a misconfigured server cannot start silently.

| test | what it checks |
| --- | --- |
| valid full config | parses correctly, all fields populated |
| defaults applied | zero values → 30s / 1MB / 100KB |
| empty secrets block | fatal error |
| empty tools block | fatal error |
| `docker_secret` name empty | fatal error |
| `description` empty | fatal error |
| `base_url` empty | fatal error |
| `base_url` contains path component | fatal error (`/v1` suffix rejected) |
| `base_url` non-https without `http: true` | fatal error |
| `base_url` non-https with `http: true` | accepted |
| `base_url` https with `http: true` | accepted (flag means "allow http", not "require http") |
| inject references unknown secret | fatal error |
| inject `secret` field empty | fatal error |
| malformed YAML | fatal error |
| file not found | fatal error |

## `internal/secrets` — `secrets_test.go`

Use `os.WriteFile` to temp files; override the `/run/secrets/` prefix by writing
files into `t.TempDir()` and passing the dir to a test-only `LoadFromDir` helper
or by temporarily setting up the files such that `Load` can find them.

| test | what it checks |
| --- | --- |
| reads file, trims trailing newline | value is clean |
| trims all whitespace variants | `\r\n`, spaces, tabs |
| empty value after trim | fatal error |
| file missing | fatal error |
| `Store.Get` known key | returns value |
| `Store.Get` unknown key | panics with clear message |
| `Store.Names` | sorted, names only |
| `Store.Values` | all values present |

## `internal/proxy` — `proxy_test.go`

This is the highest-value test file. `validateAndJoinURL` and `scrubBody` are
security-critical and purely functional — table-driven tests, no server needed.
`Do` tests use `httptest.NewServer`.

### `validateAndJoinURL` — table-driven

| input path | expected outcome |
| --- | --- |
| `/repos/owner/repo` | joined correctly with base host |
| `repos/owner/repo` (no leading slash) | slash added, joined correctly |
| `/a/../b` | normalized to `/b` |
| `/a/../../etc/passwd` | normalized, host check catches escape |
| `https://evil.com/path` | error — contains `://` |
| `//evil.com/path` | error — has host |
| empty string | error |
| path with query string `?foo=bar` | query preserved in final URL |

### `scrubBody`

| scenario | expected outcome |
| --- | --- |
| body contains no secret | body returned unchanged |
| body contains a secret value verbatim | replaced with redaction notice |
| body contains partial secret (not full match) | body returned unchanged |
| empty body | empty string returned |

### `allowedHeaders`

| scenario | expected outcome |
| --- | --- |
| response has `Content-Type` | included in result |
| response has `X-RateLimit-Remaining` | included |
| response has `X-Ratelimit-Limit` (varied case) | included |
| response has `Authorization` | dropped |
| response has `Set-Cookie` | dropped |
| response has no allowed headers | empty map returned |

### `Do` — via `httptest.NewServer`

| scenario | what it checks |
| --- | --- |
| redirect response (301) | returned as-is, not followed |
| request body over limit | error before network call |
| response body over limit | error after reading |
| injected header arrives at server | server receives correct `Authorization` |
| agent-supplied injected header dropped | agent `Authorization` overwritten by injected value |
| blocked header `Connection` stripped | server does not receive it |
| response body contains secret value | body replaced with redaction notice |
| server returns 404 | `Response.Status` is 404, no error |
| context cancelled | error returned |
