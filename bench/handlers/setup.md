# manifest

The tool discovers and installs project dependencies in parallel by matching manifest files and setup scripts.

# Types

# Functions

## Handler.HandleSetup(_ context.Context, req) (*mcp.CallToolResult, error)

1. Require a paths array of strings.
2. For each path, build the manifest command and locate a setup script.
3. Combine the setup script and manifest command when both are present.
4. Submit the command as a background job, tagging the job ID and script.
5. Report an error when no supported rule is found.

## buildManifestCommand(projectPath) string

1. Stat each known manifest file and append its command.
2. Join the commands with &&, returning empty when none match.

## findSetupScript(projectPath) (string, error)

1. Stat each candidate in order and return the first regular file.

#### Rationale

- Manifest commands run in a fixed order so multiple package managers can be satisfied by a single command.
- The setup script runs before the manifest command so it can prepare the environment.
