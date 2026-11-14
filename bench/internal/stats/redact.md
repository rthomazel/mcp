# Redaction

The package replaces known secret patterns with markers, collecting the byte counts of the original matched text.

# Types

# Functions

## pass2Redact(cmd, userPatterns) (string, []int)

1. Redact environment assignments.
2. Redact flag values in = and space forms.
3. Redact URL credentials, bearer tokens, JWTs, PEM blocks, UUIDs, long hex, base64, emails, and public IPs.
4. Apply each user pattern.

## redactEnvAssigns(cmd, counts) string

1. Redact an environment assignment only when the name is sensitive.

## redactFlagValues(cmd, counts) string

1. Redact a = flag value only when the flag name is sensitive.

## redactFlagValuesSpace(cmd, counts) string

1. Redact a space-separated flag value only when the flag name is sensitive.
2. Skip values that look like another flag.

## redactURLCreds(cmd, counts) string

1. Redact the credentials portion of a URL, keeping the scheme.

## redactBearerTokens(cmd, counts) string

1. Redact the bearer token, keeping the scheme.

## redactJWTs(cmd, counts) string

1. Replace the JWT with a marker.

## redactPEMBlocks(cmd, counts) string

1. Replace the PEM block with a marker.

## redactUUIDs(cmd, counts) string

1. Replace the UUID with a marker.

## redactLongHex(cmd, counts) string

1. Replace the long hex with a byte-counted marker.

## redactBase64(cmd, counts) string

1. Find runs of at least 24 base64-alphabet characters with optional padding.
2. Skip runs that start with a slash, to keep absolute and relative paths.
3. Skip runs that do not mix upper case, lower case, and digits.
4. Replace the rest with a byte-counted marker.

#### Rationale

- The slash and character-mix filters trade missed redaction of unusual base64 values for not mangling paths and identifiers.

## redactEmails(cmd, counts) string

1. Replace the email with a marker.

## redactPublicIPs(cmd, counts) string

1. Skip private and loopback addresses.
2. Replace public addresses with a marker.

## isPrivateIPv4(octets) bool

1. Classify the four octets as private, loopback, link-local, or reserved.

## redactCustom(cmd, pattern, counts) string

1. Replace the pattern match with a marker, appending the byte count.

## isSensitiveVarName(name) bool

1. Match the name exactly, as a suffix, as a prefix, or as an embedded component.

#### Rationale

- Redaction tiers run in order so earlier matches take precedence.
- Private addresses are excluded so routine commands are not over-redacted.
