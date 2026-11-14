# Types

## Views

Each tool result is a JSON object built from small view types, one per domain type returned: Instrument, Transaction, Position, IntegrityError, Holding, Portfolio, PriceBar, SnapshotRow, SnapshotDay, RefreshResult, RebuildResult and SnapshotIssue.

Conventions shared by every view:

1. Keys are snake_case versions of the domain field names.
2. Decimals are strings carrying the exact stored text; no number is ever a float.
3. Dates are YYYY-MM-DD strings and timestamps are RFC 3339.
4. Optional values that are absent are null, never zero or an empty string.
5. Instruments appear as symbol and exchange; internal ids are not exposed.

The views keep the domain package free of presentation tags, since Date is a plain time value that would otherwise marshal as a timestamp.

# Functions

## formatDate(date Date) string

Render date as YYYY-MM-DD.

## formatOptionalDate(date *Date) *string

Return nil for nil, otherwise the formatted date.

## toSnapshotDays(rows []domain.SnapshotRow) []SnapshotDay

Group rows, which arrive ordered by date, into one SnapshotDay per date carrying the date, its total and its rows in order.

The remaining constructors map a domain value to its view field by field, naming each instrument by symbol and exchange where the domain value holds an id.
