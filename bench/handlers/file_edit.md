# Shared file-editing helpers

The package provides file-edit plumbing: symlink resolution, file opening, commit, replacement input guards, and error builders. Proposed file_insert and file_delete reuse openFileForEdit and commit through file_cursor.md; they do not use replacement-specific validation or match diagnostics. file_create also reuses resolveTarget.

# Types

## editedFile

1. realPath string
2. content string
3. checksum [32]byte
4. lines int
5. mode os.FileMode
6. lock *file.LockEntry

# Functions

## resolveTarget(path) (string, error)

1. Lstat the path to determine whether the final element is missing.
2. Resolve the parent directory when the final element is missing.
3. Otherwise resolve the path and return the real path.

## openFileForEdit(path) (*editedFile, error)

1. Resolve the target, returning an error on failure.
2. Stat the target, rejecting a missing or non-regular file. Proposed diagnostic cleanup: report file does not exist without replacement-specific find-not-found wording for every caller.
3. Acquire the per-file lock.
4. Read the content, rejecting null bytes and invalid UTF-8.
5. Return the opened file.

## (ef *editedFile) commit(working, dryRun) (result, error)

1. Return the diff on dry-run without writing.
2. Re-read the file and abort when an external modification changed the checksum.
3. Atomically write the working content at the recorded mode.
4. Return the diff.

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

- openFileForEdit takes the lock before reading so a concurrent edit cannot slip in.
- The checksum re-check in commit detects an external modification between the read and the write.
