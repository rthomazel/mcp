---
id: 2026-11-23-duration
type: spec
summary: Readable calendar durations such as 5 days, 3mo or 1y2mo for configuration settings, with one parser and fixed rules shared by every setting that names a span of time.
author: Thom
created: 2026-11-23
agents: claude
---

# Durations in configuration

## Description

Settings that name a span of time, such as the window, interval and look-back of the monitors,
are written as short readable text and parsed by one shared parser, so every setting accepts the
same forms and rejects the same mistakes. The operator may write 5 days, 12 months, 3mo or 1y2mo.
The system works on daily data and calendar dates, so the finest unit is the day.

The standard library's duration parser knows only hours, minutes and seconds and has no day, week,
month or year, so the parser is the project's own, small by design: few forms, fixed rules.

## Concepts

**Duration.** A calendar span made of years, months, weeks and days. It is not a fixed number of
hours: months and years have different lengths, so a duration only has a length when it is applied
to a date.

**Component.** A whole number followed by a unit.

**Units.** Each has a short form and long forms, and the singular and plural are interchangeable
whatever the number, so 1 days and 2 day are valid.

| Unit  | Short | Long forms     |
| ----- | ----- | -------------- |
| day   | d     | day, days      |
| week  | w     | week, weeks    |
| month | mo    | month, months  |
| year  | y     | year, years    |

A week is seven days. Units are case-insensitive.

## Behavior

### Accepted forms

There are two forms and nothing else.

- **One component**, in any unit form: a number, optional spaces, then the unit. Examples: 5d,
  5 d, 5 days, 5days, 1 year, 12 months, 3mo, 2 WEEKS.
- **Several components**, in short units only, written together with no spaces: two or more
  components, each unit at most once, in any order. Examples: 1y2mo, 1y2mo3d, 2w3d, 3d1w.

Spaces before and after the text are ignored. Between the number and the unit of a single
component any number of spaces is allowed; nowhere else.

### Rules

- A number is a whole number of one to four digits, with no sign, no fraction and no separators.
  Leading zeros are allowed: 05d is five days.
- The total must be greater than zero. Zero components are allowed beside a positive one, so
  1y0d is one year, and 0d or 0y0mo are rejected.
- A bare number with no unit is always rejected, whether written as text or as a YAML number, so
  an unclear 15 never silently means days.
- An empty value is rejected. A setting that is left out takes its own default; one that is
  written empty does not.

### Calendar arithmetic

A duration is applied to a calendar date, the market date, with no time of day.

- Counting back from a date, years and months are applied first and then weeks and days, so
  1mo2w back from 20 March is 20 February, and then 6 February.
- When the day of the month does not exist in the target month, the result is the last day of
  that month: one month before 31 March is 28 February, or 29 in a leap year; one year before
  29 February is 28 February. It never spills into the next month.
- Counting forward uses the same rules.
- Durations with the same total in days are equal in effect: 7d and 1w are the same, and 12mo and
  1y give the same date. 30d and 1mo are not the same.
- The text is kept as the operator wrote it, trimmed, for messages and logs.

### Errors

Every rejection names the setting, the text as written and the reason in plain words, for example
that a unit is missing, that the unit is not known, that a unit is repeated, that the number is
too long, that the duration is zero, or that forms cannot be mixed. A value with several mistakes
reports the first.

### When it is checked

The text is parsed when the configuration is validated, never when a monitor runs, so an invalid
duration is an invalid file. At server start an invalid file stops the server with the error; the
monitors configuration describes this and the live reload that follows.

## Cases

Accepted:

| Text           | Meaning                          |
| -------------- | -------------------------------- |
| 1d             | one day                          |
| 5 days         | five days                        |
| 1 day          | one day                          |
| 2 day          | two days, singular accepted      |
| 3days          | three days, no space             |
| 1w, 2 weeks    | seven and fourteen days          |
| 3mo, 12 months | three and twelve months          |
| 1y, 1 year     | one year                         |
| 2 WEEKS, 1Y    | upper case                       |
| 1y2mo          | one year and two months          |
| 1y2mo3d        | one year, two months, three days |
| 3d1w           | any order                        |
| 05d            | leading zero, five days          |
| 1y0d           | zero beside a positive component |
|  5d            | surrounding spaces ignored       |
| 9999d          | the longest number               |

Rejected:

| Text                              | Reason                                     |
| --------------------------------- | ------------------------------------------ |
| 15, 15.0 (number or text)         | no unit                                    |
| empty, spaces only                | empty                                      |
| d, days                           | no number                                  |
| 0d, 0y0mo                         | total is zero                              |
| -5d, +5d                          | sign                                       |
| 1.5y, 0.5d                        | fraction                                   |
| 1,000d, 1_000d                    | separator                                  |
| 10000d                            | number longer than four digits             |
| 72h, 90m, 30s, 1h30m              | unit not supported, the finest is the day  |
| 1q, 2 quarters, 1 fortnight       | unit not known                             |
| 1y 2mo, 1 year 2 months           | several components must be written together in short units |
| 1year2months                      | long units only in a single component      |
| 1y1y, 2d3d                        | repeated unit                              |
| 1d2                               | trailing number with no unit               |
| 5 d ays, 5 dd                     | unit not known                             |
| 5 days ago, past 5d               | extra words                                |
| 1y,2mo                            | separator                                  |
| 5d; 5d                            | extra text                                 |
| P1Y2M                             | ISO 8601 is not supported                  |
| ５d (full-width digit)            | not a plain digit                          |

## Not covered

These are left out on purpose; a value like these is rejected. Say if one should be added.

- Hours, minutes, seconds and the standard library's forms such as 72h.
- Fractions of a unit, such as 1.5y or half a day.
- Trading or business days, such as 5 trading days. A duration is always calendar time.
- Several components with spaces, commas, long names or the word and.
- Quarters, fortnights, decades and other units.
- Signs, so there are no negative durations, and words like ago or past.
- ISO 8601 durations.
- Other languages and non-ASCII digits.
- A maximum total: only the four digit limit applies, so 9999y is accepted.
- A unit shorter than the data's granularity: nothing below one day exists.
- The special values none, off, forever or all. A setting that needs to be off is disabled in
  its own way.
- Reading a duration anywhere but configuration, such as in an agent tool's arguments.

## Acceptance criteria

- Every accepted case above parses and every rejected case is rejected, each with an error that
  names the setting, the text and the reason.
- A YAML number given for a duration is rejected as having no unit.
- One month before 31 March is 28 February; one year before 29 February 2028 is 28 February 2027;
  1mo2w before 20 March is 6 February.
- 7d and 1w apply identically; 12mo and 1y apply identically.
- An invalid duration in the monitors file stops the server at start and is logged; the same
  edit on a running server keeps the last valid content, as the monitors configuration says.

## Decisions

- Calendar time, because the settings are the operator's and a month or a year is how they think
  about it, not trading days.
- One parser with two forms and no mixing, to keep it small and the error messages simple.
- Month and year arithmetic clamps to the end of the month, so a date is never moved forward past
  the month it was counting into.
- Not accepting a bare number removes any doubt about the unit.
- Parsed at validation so an error is found at startup and never at the 7:00 AM run.

## Refs

- Related: 2026-11-22-monitors-configuration, which validates settings that use durations.
- Related: 2026-11-23-monitor-portfolio-movement, 2026-11-23-monitor-price-volatility and
  2026-11-23-monitor-portfolio-extremes, which use window, interval and look-back settings.
