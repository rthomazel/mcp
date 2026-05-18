# jail-mcp — agent context

Read `doc/` .md files for architecture and project documentation before making changes — it's faster than reading source.

## guidelines

- Run `go mod tidy` after any `go.mod` or dependency changes
- Run the formatter (`gofumpt`) as the last step after code changes
- Do not document obvious things
- Be minimalistic: give the right answer, avoid guessing or workarounds; if blocked, say so explicitly
- Avoid single-letter variable names unless scope is very small (receivers and loop vars are fine)
- Avoid multi-line `if` conditions with `samber/lo` functions
- When refactoring, minimize renames unless asked
- Add tests only when asked; focus on code that is complex or prone to bugs
- Write functions in call order — entry point first, then what it calls
- Do not start background jobs on your own; wait to be asked
- Go packages should have a doc.go file with a // Package Foo ... go doc command
