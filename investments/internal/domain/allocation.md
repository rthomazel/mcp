# Constants

SideAbove = "above", SideBelow = "below", the sides of a band.

# Types

## AllocationGroup

1. Name string, unique among the groups
2. Tickers []string, upper case
3. MinPercent *Decimal, nil when the group has no minimum
4. MaxPercent *Decimal, nil when the group has no maximum

A named set of instruments judged as one against a band, built from the monitor's settings.

## MemberAllocation

1. Symbol string
2. AllocationPercent Decimal, the instrument's share of the securities value

Serialized in event payloads.

## GroupAllocation

1. Name string
2. AllocationPercent Decimal, the group's share of the securities value on the latest date
3. Side string, SideAbove or SideBelow, empty when inside the band
4. StayStart *Date, the first date of the current stay on Side, nil when inside the band
5. Members []MemberAllocation, the registered members, ordered by symbol

Above means the share is greater than the maximum, below means it is less than the minimum; a share exactly at a bound is inside. The stay start is the day after the most recent date on a different side, including inside the band, or the first date of the series when there was none.

## AllocationDrift

1. Date Date, the latest date of the securities series
2. Groups []GroupAllocation, in the order of the input groups
3. Unregistered []string, tickers with no registered instrument, each once

Every group's allocation on the latest date. Unregistered tickers count as zero. A share of the securities value, so trades change it as well as prices.
