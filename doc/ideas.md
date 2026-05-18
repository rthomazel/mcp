# ideas

## concurrent context

`context` tool runs subprocesses serially.
Could run them with goroutines and be meaningfully faster.

## per-command timeout

Timeout is global via `JAIL_MCP_TIMEOUT`.
Letting `exec_sync` accept an optional `timeout` param would be useful for known slow commands.

## sqlite db with command stats

Server would tokenize commands with weights, base command has higher weight, then flags.
Normalize input.
Expose historic command stats to allow planning when to use exec sync or background.

# what not to add

- filesystem MCP tool — redundant, shell already does cat/ls/cp/find
- command allowlists — defeats the purpose, Docker is the boundary

## indented xml output

`xmlBuilder` has no depth tracking. A `depth int` field incremented by `openTag` /
decremented by `closeTag` would let all write methods prepend indentation.
Metadata fields written directly via `WriteString` would need a `b.line(s)` helper
to respect depth — a wider refactor touching all handlers.

## path snapshot registration file

Setup scripts could write a `.jail-mcp-extras` file in the project root —
one `name: /path/to/binary` pair per line. `context` reads all such files under
known project roots and surfaces them alongside the `auto-detected in path:` block.
Explicit opt-in, works for non-PATH installs, but requires setup scripts to be
authored with the convention.

## command variables

We could have a variable, a plaintext constant that the model can provide into the shell maybe or the github tool and it will get replaced by the mcp with the actual token.
That way we keep the token out of the context, and don't overbuild this.
Actually doing this in Jail, MCP would be so easy. There could be a new tool called load variable and then the model passes in a string. It can even pick whatever string it wants. It also passes the environment variable to read in the server to retrieve the value. The server could reply with messages variable is not set, variable is empty and those would be errors but if the variable is set and not empty it would reply with a success value and the checksum of the value.
Having the checksum gives the model the ability to use the value without knowing the value.
Then the model can use the other tools in any way that it wants. When the server sees a string compatible with a variable, it replaces in the token before executing.
We should probably have some type of exclusive string so that we can catch unset variables in commands and return an error and refuse to process the command.
We could also have a tool to list the variables that are currently loaded in the server.
A design question is, should this live in the jail or be a separate MCP tool? This design kind of couples the variable idea to another tool that executes commands. So I think this has to be done in the jail.
The obvious upside is that it's so easy and simple. 