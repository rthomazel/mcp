# Constants

maxDigits = 4, the most digits of a number.
hoursPerDay = 24, daysPerWeek = 7, monthsInYear = 12.
daysPerMonth = 30.4375, the length of a month when two durations are compared.

# Vars

## units

The five units, each with its short form and long forms: h with hour and hours, d with day and days, w with week and weeks, mo with month and months, y with year and years.

## subDayUnits

Unit words shorter than a day: ms, s, sec, secs, second, seconds, m, min, mins, minute, minutes. They are recognised only to be rejected with their own reason.

# Types

## Duration

1. Months int, years counted as twelve months each
2. Days int, weeks counted as seven days each and hours as a twenty-fourth
3. text string, the trimmed text as written

A calendar span with no length until applied to a date. Equal totals are equal in effect: 1w and 7d, 1y and 12mo.

## component

1. number int
2. unit string, the short form
3. long bool, whether a long unit name was written
4. spaced bool, whether spaces separated the number and the unit

# Functions

## Parse(setting string, text string) (Duration, error)

1. Trim spaces from text and lower-case it for parsing.
2. Parse the lower-cased text with parse().
3. Keep the trimmed original as the Duration's text.
4. Return the Duration.

#### Errors

- **2.** return an error naming the setting, the text as written and the reason from parse().

The error format is: setting "{setting}": duration "{text}" is invalid: {reason}.

## Duration.String() string

Return the trimmed text as written.

## Duration.Compare(other Duration) int

Return -1, 0 or 1 comparing Months times daysPerMonth plus Days. Approximate, only for checking that one span is not shorter than another, such as a look-back against a window: 1y is longer than 365d and 1mo is between 29d and 31d. It never moves a date.

## Duration.Before(date Time) Time

Return shift(date, -1).

## Duration.After(date Time) Time

Return shift(date, 1).

## Duration.shift(date Time, sign int) Time

1. Move the month index by sign times Months, carrying into the year with floor division.
2. if the day does not exist in the target month, use the last day of that month.
3. Rebuild the date with the same time of day and location.
4. Add sign times Days days.
5. Return the result.

Months are applied before days, so 1mo2w before 20 March is 20 February and then 6 February, and one month before 31 March is 28 February.

## parse(s string) (Duration, error)

1. if s is empty, return the empty reason.
2. Split s into components with scan().
3. if there are several components, require every one to have no space between number and unit and to use a short unit.
4. Return total() of the components.

#### Errors

- **3.** if a component is spaced, return the reason that several components must be written together.
- **3.1.** if a component used a long unit, return the reason that long names are for a single component only.

## scan(s string) ([]component, error)

1. Repeat until the text is consumed.
2. Read ASCII digits; at most maxDigits, then compute the number.
3. Skip spaces after the digits, remembering whether there were any.
4. Read the letters of the unit and resolve them with lookup().
5. if spaces follow the unit, fail: another number after them means mixed forms, anything else is extra text.

#### Errors

- **2.** if there are no digits, return unexpected(): a letter is a missing number, a sign, fraction and separator have their own reasons.
- **2.1.** if there are more than maxDigits digits, return the too-long reason.

---

- **4.** if there are no letters, return the missing-unit reason at the end of the text, otherwise unexpected().

## unexpected(s string, i int) error

Return the reason for the character at i: missing number for a letter, a sign, a fraction, a separator, or unexpected text for anything else, including non-ASCII digits.

## lookup(word string) (short string, long bool, err error)

Resolve a unit word, singular or plural, to its short form and whether it was a long name. A word in subDayUnits returns the shorter-than-a-day reason, any other unknown word the unit-not-known reason.

## total(comps []component) (Duration, error)

1. For each component, fail if its unit was already seen, then add it: years and months to Months, weeks and days to Days, hours kept apart.
2. if the hours are not a multiple of hoursPerDay, fail.
3. Add the hours as whole days.
4. if Months and Days are both zero, fail.
5. Return the Duration.

#### Errors

- **1.** return the repeated-unit reason.

---

- **2.** if the hours are below a day and nothing else is present, return the shorter-than-a-day reason, otherwise the whole-days reason.

---

- **4.** return the zero reason.
