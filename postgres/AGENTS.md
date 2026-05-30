# postgres-mcp — agent context

Read `doc/design.md` before making changes — it covers all tool groups, configuration, transaction model, and known v1 limitations.

## guidelines

- Run `go mod tidy` after any `go.mod` or dependency changes
- Run the formatter (`gofumpt`) as the last step after code changes
- Do not document obvious things
- Be minimalistic: give the right answer, avoid guessing or workarounds; if blocked, say so explicitly
- Avoid single-letter variable names unless scope is very small (receivers and loop vars are fine)
- When refactoring, minimize renames unless asked
- Add tests only when asked; focus on code that is complex or prone to bugs
- Write functions in call order — entry point first, then what it calls
- Go packages should have a doc.go file with a `// Package Foo ...` go doc comment

## sql validation

All SQL keyword validation lives in `internal/sqlcheck`. Touch that package when adding new allowlisted tokens or changing multi-statement detection. Run `go test ./internal/sqlcheck/...` after any change there.

## adding a tool

1. Add the handler method to the appropriate file in `handlers/`
2. Register the tool in `main.go` with `s.AddTool`
3. Update `doc/design.md` if the tool changes the public API
