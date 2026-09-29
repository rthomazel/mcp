# XML builder

The package renders XML tags with attributes and optional newlines.

# Types

## Builder

1. strings.Builder

# Functions

## Builder.OpenTag(tag, attrs ...)

1. Write the tag name.
2. Write each attribute as key-value pairs.
3. Write the opening bracket.

## Builder.CloseTag(tag, newline)

1. Write the closing bracket.
2. Optionally write a trailing newline.

## Builder.Tag(name, contents, newline, attrs ...)

1. Open the tag.
2. Trim the trailing newlines from the contents.
3. Write the contents and a newline.
4. Close the tag.

#### Rationale

- Trimming the trailing newlines keeps the content readable without stray blank lines.
