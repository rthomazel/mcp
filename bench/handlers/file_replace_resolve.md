# file_replace resolution

Per-hunk resolution: exact, loose, or unmatched. A replacement resolves by first trying an exact match scoped to its region; when none exists it tries a loose match scoped to its region, classifying the hit by how it was derived; and when neither resolves it is reported with diagnostics.

## resolveReplacement(r, content, fileLines, maxCandidates) *resolution

1. Try resolveExact(r, content) to locate a single, unambiguous exact match.
2. If it returns exactly one match, return a resolution with outcome "exact" and that match.
3. Otherwise try resolveLoose(r, content, fileLines) to locate a single candidate.
4. If it returns a match and kind, return a resolution with outcome "candidate" and the loose candidate.
5. Otherwise return a resolution with outcome "unmatched" and its diagnostics, built by findResolvedDiagnostics.

## Resolution

1. outcome string, one of "exact" | "candidate" | "unmatched"
2. located *file.Match, the exact match when outcome is "exact"
3. loose *looseCandidate, the derived match when outcome is "candidate"
4. diagnostics string, the preview when outcome is "unmatched"

## resolveExact(r, content) *file.Match

1. Gather all exact matches of r.find in content.
2. Narrow to those overlapping [startLine, endLine] when the region is set.
3. Return nil when zero matches remain.
4. Return the sole match when exactly one remains.
5. Return a non-nil sentinel when more than one remains, signaling ambiguity.

## resolveLoose(r, content, fileLines) (*looseCandidate, string)

1. Find the anchors — region, prefix, and indentation — that may scope the search.
2. If startLine or endLine is set, compute the region span, clamping endLine to fileLines, and return unmatched when the region is empty.
3. If a prefix is set, match it across the region; when it has multiple hits, return unmatched and report the ambiguity.
4. If no prefix, match the first non-empty line of find across the region; when it has multiple hits, return unmatched and report the ambiguity.
5. Classify the single remaining hit: "region" when it was chosen purely by region, "prefix" when a prefix drove it, or "indent" when an indentation drift drove it.
6. Return the loose candidate and an empty diagnostics string.

## findResolvedDiagnostics(label, r, content, fileLines) string

1. When the region is set, report the region span and a snippet.
2. Fall back to partialMatchDiagnostic when the first non-empty line of find matches somewhere.
3. When find starts with the anchor line, report the char offset of the mismatch so a drift is visible.
4. Report the original find and the anchor line for comparison.

#### Rationale

- Anchors are gathered in priority order (region, prefix, indent) so a caller can be as loose or as precise as it wants.
- Returning a sentinel instead of an error for an ambiguous exact match lets resolveReplacement fall through to a loose attempt, which is the forgiving behavior the spec asks for.
- findResolvedDiagnostics turns a non-match into a rendered preview so the model sees exactly what it tried, in the same diff-shaped artifact as success.
