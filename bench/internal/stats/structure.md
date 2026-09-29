# Structural normalization

The package replaces shell and interpreter constructs with byte-counted markers so their content does not trigger spurious long-token matches.

# Types

# Functions

## pass3Structural(cmd, counts) (string, []int)

1. Normalize inline scripts.
2. Normalize Python blocks.
3. Normalize heredocs.
4. Normalize herestrings.
5. Normalize process substitutions.
6. Normalize subshells and backtick subshells.

## normalizeInlineScripts(cmd, counts) (string, []int)

1. Replace python, perl, ruby, node, awk, sed, and their -c/-e invocations with a byte-counted marker.

## normalizePythonBlocks(cmd, counts) (string, []int)

1. Replace triple-quoted Python blocks with a byte-counted marker.

## normalizeHeredocs(cmd, counts) (string, []int)

1. Find the introducer and delimiter.
2. Sum the body byte length, skipping the terminator.
3. Replace the block with a byte-counted marker.

## normalizeHerestrings(cmd, counts) (string, []int)

1. Trim the delimiters and replace the string with a byte-counted marker.

## normalizeProcessSubs(cmd, counts) (string, []int)

1. Replace the process substitution with a marker.

## normalizeSubshells(cmd, counts) (string, []int)

1. Replace the $(...) and backtick subshells with a marker.

#### Rationale

- Structural runs before long-token normalization so a long command body does not inflate the long-token counts.
- An unterminated heredoc consumes the remainder rather than risk a bad split.
