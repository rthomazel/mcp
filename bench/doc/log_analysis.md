### Design Plan — Log Analysis MCP Tool

## Precondition

This tool is only invoked for failed jobs with non-empty logs. Successful jobs (exit code `0`) and empty logs bypass this tool entirely.

---

## Config (with defaults)

```text
context_budget_bytes: int  // derived from model's context window minus prompt overhead
llm_timeout_seconds: int = 3
max_relevant_lines: int = 50
```

---

## Pipeline

1. Receive raw log.
2. Redact the log using the existing redaction code.
3. Number the **redacted** log lines. These global line numbers become the canonical source of truth.
4. Split the numbered log into one or more contiguous chunks that each fit within `context_budget_bytes`. Never split within a line.
5. Send each chunk independently to the small LLM (temperature 0, structured output, 3s timeout), expecting `{diagnostic, relevant_line_indices}`.
6. Validate each response:

   * JSON parses.
   * Required fields exist.
   * `diagnostic` is present (may be empty).
   * `relevant_line_indices` is an array of integers.
   * Parse failure, timeout, or validation failure → retry once with reinforcement appended to the user message.
   * Second failure of any kind → return an empty terminal result for that chunk and log a warning.
7. On successful validation:

   * Filter `relevant_line_indices` to `[1, N]`; log a warning listing any dropped out-of-range indices.
   * Deduplicate indices.
   * Sort ascending.
   * Cap to `max_relevant_lines`; log a warning if indices were dropped due to the cap.
8. Look up surviving indices against the canonical numbered log to assemble byte-exact verbatim lines.
9. Return one analysis object per chunk.

---

## Output contract

```text
{
  analyses: [
    {
      success: bool,
      diagnostic: string,
      relevant_lines: [{index, text}]
    }
  ]
}
```

A chunk with no detected failure:

```text
{
  success: true,
  diagnostic: "",
  relevant_lines: []
}
```

Terminal failure for a chunk (after retry):

```text
{
  success: false,
  diagnostic: "",
  relevant_lines: []
}
```

Interpretation:

* `success = false` → analysis failed (timeout, invalid response, etc.).
* `success = true` and empty `diagnostic` → no failure found in this chunk.
* `success = true` and non-empty `diagnostic` → failure detected.

---

## System prompt

```text
You are a build/test log analyzer. You will be given numbered lines from a
log (a compiler, linter, test runner, or similar build tool). Your job is
to identify the underlying error(s) and explain them clearly.

Rules:
- Respond ONLY with a single JSON object matching the schema below. No
  preamble, markdown fences, or commentary.
- Diagnose the ROOT CAUSE, not just the first or last error shown. If one
  failure cascades into many downstream errors, identify the originating
  one and say so explicitly.
- If the chunk contains no failure or diagnostic information, return an
  empty string for "diagnostic" and an empty array for
  "relevant_line_indices".
- The "diagnostic" field must be plain text, 2-4 sentences: what failed,
  why (as far as the log shows), and what part of the codebase or
  configuration is implicated.
- The "relevant_line_indices" field must contain the line numbers (as
  shown in the "N: " prefix of each input line) that best support the
  diagnosis. Include all surrounding context lines that materially help a
  developer understand the failure, such as stack traces, compiler notes,
  assertion context, or related multi-line messages. Prefer the smallest
  useful set, typically 3-15 lines, but include more when necessary for
  clarity. Do not invent line numbers. Do not include line text in your
  response, only integers.

JSON schema:
{
  "diagnostic": string,
  "relevant_line_indices": number[]
}
```

---

## User prompt (templated)

```text
Job command line:
{{job_command_line}}

Log chunk (line-numbered, redacted):
{{numbered_log_chunk}}
```

---

## Retry message (appended to the same user message on any failure)

```text
Your previous response did not satisfy the required schema. Respond again
with ONLY the JSON object described above. Do not include markdown,
code fences, or explanatory text.
```

### Notes

* Line numbers are global across the entire log, not per chunk.
* The application, not the LLM, retrieves the final log lines verbatim.
* Chunking is purely a transport mechanism to fit within the model's context window; callers receive one independent analysis per chunk and may aggregate or present them as appropriate.
* Independent chunks can be processed sequentially today and parallelized later without changing the API.
