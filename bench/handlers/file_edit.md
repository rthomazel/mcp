# Shared file-editing helpers

The package provides file-edit plumbing: symlink resolution, file opening, commit, replacement input guards, and error builders. file_insert and file_delete reuse openFileForEdit and commit through file_cursor.md; they do not use replacement-specific validation or match diagnostics. file_create also reuses resolveTarget.

# Types

## editedFile

1. realPath string
2. content string
3. checksum [32]byte
4. lines int
5. mode os.FileMode
6. lock *file.LockEntry

# Functions

## resolveTarget(path string) (realPath string, toolErr string)

1. Call os.Lstat() to inspect the final element.
   1. if it is missing, resolve the parent with filepath.EvalSymlinks(), then return filepath.Join() of the resolved parent and original base name.
2. Otherwise resolve the complete path with filepath.EvalSymlinks() and return it.

#### Errors

- **1.1.** if parent resolution reports a missing path, return a parent-directory-does-not-exist diagnostic.
- **1.1.** if parent resolution otherwise fails, return the contextual resolve-path diagnostic.

---

- **2.** if complete-path resolution fails, return the contextual resolve-path diagnostic. Non-missing Lstat failures proceed to this resolution, matching the existing helper.

## openFileForEdit(path string) (opened *editedFile, toolErr string)

1. Resolve the target with resolveTarget().
2. Stat the target and inspect its type.
3. Acquire the per-file lock with file.AcquireLock().
4. Read content with os.ReadFile() and validate it.
5. Return the opened file with its content, checksum, mode, line count, and owned lock.

#### Errors

- **1.** if resolution fails, return its diagnostic.

---

- **2.** if the file is missing, return file does not exist. Uses tool-neutral wording for all callers.
- **2.** if stat otherwise fails, return the contextual stat error.
- **2.** if the target is non-regular, return a file-type error.

---

- **4.** if reading fails, release the lock with file.ReleaseLock() and return the contextual read error.
- **4.** if content contains null bytes or invalid UTF-8, release the lock with file.ReleaseLock() and return the binary-content error.

## editedFile.commit(working string, dryRun bool) (result string, toolErr string)

1. Inspect dryRun.
   1. if true, return file.ComputeDiff() without writing.
2. Re-read the file and compare its checksum to the captured checksum.
3. Call file.AtomicWrite() with working content and the recorded mode.
4. Return file.ComputeDiff().

#### Errors

- **2.** if re-reading fails, return the contextual re-read error without writing.
- **2.** if the checksum differs, return the external-modification error without writing.

---

- **3.** if atomic writing fails, return the contextual write error rather than a success diff.

The caller retains ownership of the lock and releases it on every return. Dry-run validates the captured snapshot; checksum revalidation is a write-time guard, not part of preview validation. Successful no-ops likewise skip commit and write-time revalidation.

## validateFindReplace(find, replace, maxLines) error

1. Reject an empty find.
2. Reject an identical find and replace pair.
3. Reject null bytes in either string.
4. Reject invalid UTF-8 in either string.
5. Reject a replace exceeding the newline limit.

## partialMatchDiagnostic(find, content, maxCandidates) (hint, error)

1. Take the first non-empty line of find.
2. Search for it anywhere in content.
3. Return a hint when it matches, otherwise an empty hint.

## zeroMatchError(label, find, content, maxCandidates) error

1. Build the diagnostic when find matches zero times.

## multiMatchError(label, find, content, maxCandidates) error

1. Build the diagnostic when find matches more than once.

#### Rationale

- The process-local lock serializes cooperating edits after acquisition; target type and mode are inspected before locking.
- The checksum re-check detects external changes visible at re-read, not changes occurring between that check and atomic replacement. It does not exclude external writers.
