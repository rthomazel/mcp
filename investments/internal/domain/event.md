# Types

## EventHandlers = func(eventType string) []EventHandler

The handlers registered for an event type. None means no handler is registered. A function and not a map, since the set of types a source covers can be open, such as every type with the notification prefix.

## EventPolicy

1. MaxFailures int, the failed attempts after which an event is given up on
2. BackoffBase Duration, the delay after the first failure; each further failure doubles it
3. MaxAge Duration, how long after it was recorded an event is still considered for handling; it sets the event's ExpiredAt
4. Retention Duration, how long after it was fully handled an event is kept

How the outbox retries, expires and retains events. The values are server configuration with defaults of 10 failures, 30 seconds, 7 days and 1 year, set by environment variables; the domain holds no values.

# Vars

## Events

1. Events.SnapshotIncomplete = "notification.snapshot.incomplete"
2. Events.NotificationFailed = "notification.failed"
3. Events.PortfolioDrawdown = "notification.portfolio.drawdown"
4. Events.PortfolioATH = "notification.portfolio.ath"
5. Events.PortfolioMovement = "notification.portfolio.movement"
6. Events.AllocationDrift = "notification.allocation.drift"
7. Events.PriceVolatility = "notification.price.volatility"
8. Events.PriceTarget = "notification.price.target"
9. Events.InstrumentDrawdown = "notification.instrument.drawdown"
10. Events.InstrumentATH = "notification.instrument.ath"

Event types are dot-separated words, named here and nowhere else.

## NotificationPrefix = "notification."

Starts the type of every event that produces a notification. The convention replaces a list of notifying types: the events worker selects events by it, and the notifications file may configure only types that have it. Events of other types are for other handlers, which an events worker with another prefix or none can pick up.

# Types

## Event

1. ID int64
2. Type string
3. Subject *string, nil unless a monitor recorded the event for a subject such as a date
4. Data RawMessage, a JSON object
5. Failures int
6. BackoffUntil *Time, the earliest time of the next attempt, nil before the first failure
7. CreatedAt Time
8. ExpiredAt Time, the time from which the event is expired, CreatedAt plus the policy's MaxAge, fixed when it is recorded
9. ProcessedAt *Time, set when every registered handler has succeeded
10. GivenUpAt *Time, set when Failures reached the policy's MaxFailures
11. HandledBy []string, the names of the handlers that already succeeded

One row of the outbox. An event is pending until ProcessedAt or GivenUpAt is set. It is expired when it is not fully handled and ExpiredAt is not after the reference time, whether or not it was given up on. Events are identified by ID, which status exposes because they have no natural key.

## DraftEvent

1. Type string
2. Subject *string, when set at most one event of the type exists for the subject
3. Data RawMessage, a JSON object, empty meaning an empty object

An event to record.

## EventOutcome

1. Succeeded []string, the names of the handlers that succeeded in this pass
2. Handled bool, every registered handler has now succeeded
3. Failed bool, a handler failed, or no handler was registered for the type

What one pass did with one event. Handled and Failed are never both true.

## SnapshotIncompleteData

1. Date Date, the latest price date that has no completed snapshot

Payload of Events.SnapshotIncomplete. The subject of the event is the date, formatted YYYY-MM-DD.

## PortfolioDrawdownData

1. Level Decimal, the negative percent level that was crossed
2. CrossedOn Date, the first date of the stay at or below the level
3. Date Date, the latest series date when it was recorded
4. DrawdownPercent Decimal, the drawdown on Date, zero or negative
5. ValueUSD Decimal, the securities value on Date
6. HighDate Date, the date of the all-time high
7. HighValueUSD Decimal, the securities value on HighDate

Payload of Events.PortfolioDrawdown. The subject of the event is the level, an at sign and the crossing date, for example -6@2026-11-10. The level is formatted without trailing zeros, so a level keeps its subject when the increment changes to another divisor of it.

## PortfolioATHData

1. Date Date, the date of the new high
2. ValueUSD Decimal, the securities value on Date
3. PreviousHighDate Date
4. PreviousHighValueUSD Decimal
5. IncreaseUSD Decimal, over the previous high
6. IncreasePercent Decimal, over the previous high
7. LastAnnouncedDate *Date, the date of the last announced high, nil for the first

Payload of Events.PortfolioATH. The subject of the event is Date, formatted YYYY-MM-DD.

## PortfolioMovementData

1. Date Date, the latest snapshot date
2. Direction string, up or down
3. Window string, the window as the operator wrote it
4. StartDate Date, the base date of the window
5. StartValueUSD Decimal
6. EndValueUSD Decimal
7. MarketUSD Decimal
8. MarketPercent Decimal, of StartValueUSD
9. TradesUSD Decimal
10. Gainers []Contributor
11. Losers []Contributor

Payload of Events.PortfolioMovement. The subject of the event is Date, an at sign and Direction, for example 2026-11-20@up. Date is also what the monitor's interval is measured from.

## AllocationDriftData

1. Group string
2. Side string, above or below
3. Date Date, the latest snapshot date
4. StayStart Date, the first date of the stay outside the band
5. AllocationPercent Decimal, the group's share of the securities value on Date
6. MinPercent *Decimal, the configured minimum, nil when none
7. MaxPercent *Decimal, the configured maximum, nil when none
8. Members []MemberAllocation

Payload of Events.AllocationDrift. The subject of the event is the group name, the side and StayStart, joined by at signs, for example us-core@above@2026-11-17. The bounds are the ones configured when it was recorded, so the message still reads right after a retune.

## PriceVolatilityData

1. Instrument string, the ticker
2. Date Date, the latest price date
3. Direction string, up or down
4. Window string, as the operator wrote it
5. Lookback string, as the operator wrote it, or the default that applied
6. StartDate Date, the last price date on or before the window start
7. MovePercent Decimal, the adjusted close change over the window
8. Variation Decimal, the move in standard deviations, signed
9. Threshold Decimal, the configured number of standard deviations
10. VolatilityPercent Decimal, the standard deviation of the baseline moves
11. Moves int, how many moves the volatility is taken from

Payload of Events.PriceVolatility. The subject of the event is the ticker, Date and Direction, joined by at signs, for example QQQ@2026-11-20@up. Date is also what the instrument's interval is measured from.

## PriceTargetData

1. Instrument string, the ticker
2. Price Decimal, the target, normalized
3. Date Date, the latest price date
4. Low Decimal, the day's low
5. High Decimal, the day's high
6. Close Decimal, the day's close
7. Note string, why the target exists; omitted from the JSON when empty

Payload of Events.PriceTarget. The subject of the event is the ticker and Price joined by an at sign, for example QQQ@232. Prices are unadjusted. The subject carries no date, so a target is announced once until the event is deleted, which the outbox does a year after it was handled.

## InstrumentDrawdownData

1. Instrument string, the ticker
2. Level Decimal, the level crossed
3. CrossedOn Date, the first date of the stay at or below the level
4. Date Date, the latest price date
5. DrawdownPercent Decimal, zero or negative
6. Close Decimal, the adjusted close on Date
7. HighDate Date, the date of the all-time high
8. HighClose Decimal, the adjusted close at the high

Payload of Events.InstrumentDrawdown. The subject of the event is the ticker, Level and CrossedOn joined by at signs, for example QQQ@-5@2026-11-05.

## InstrumentATHData

1. Instrument string, the ticker
2. Date Date, the date of the new high
3. Close Decimal, the adjusted close on Date
4. PreviousHighDate Date, the date of the highest earlier close
5. PreviousHigh Decimal, that close
6. IncreasePercent Decimal, the increase over it
7. LastAnnouncedDate *Date, the date of the instrument's last announced high, omitted when none

Payload of Events.InstrumentATH. The subject of the event is the ticker and Date joined by an at sign, for example QQQ@2026-11-20. Date is also what the instrument's interval is measured from.

## NotificationFailedData

1. EventID int64
2. EventType string
3. RecordedAt Time, when the failed event was recorded
4. Failures int

Payload of Events.NotificationFailed. It never carries the failed event's own payload.

# Interfaces

## EventHandler

1. Name() string
2. Handle(ctx Context, event Event) error

One side effect attached to one event type. Name is persisted in Event.HandledBy, so it is stable: never reused for different behavior, and not removed while events that name it may be in flight. A handler may run more than once for an event, since delivery is at least once.

## EventHandlerSource

1. Handlers(ctx Context) (EventHandlers, error)

The handlers registered for each event type, read fresh on every pass so configuration changes apply without a restart. Implemented by the packages that own handlers, such as notification, and used by the events worker. An error means the handlers are unavailable.

# Functions

## IsNotification(eventType string) bool

Report whether eventType starts with NotificationPrefix.

## ValidEventType(eventType string) bool

Report whether eventType is one or more words of lowercase letters, digits and underscores, starting with a letter, separated by single dots.

## DraftEvent.Validate() error

1. if Type is not valid under ValidEventType, return ErrInvalidInput.
2. if Subject is set and blank, return ErrInvalidInput.
3. if Data is not empty and is not a JSON object, return ErrInvalidInput.
4. Return nil.

Mirrors the events table checks.

## EventOutcome.Validate() error

Return ErrInvalidInput when Handled and Failed are both true.

## EventPolicy.Backoff(failures int) Duration

Return BackoffBase multiplied by two to the power failures minus one: 30 seconds after the first failure, a minute after the second, and so on, a little over an hour after the eighth and a little over two hours after the ninth. Zero for a count below one.

Never used for the last failure, which gives up instead of retrying.

## Event.Eligible(now Time) bool

1. if ProcessedAt or GivenUpAt is set, return false.
2. if Expired(now), return false.
3. if BackoffUntil is set and is after now, return false.
4. Return true.

The reference for the SQL that selects due events, and the way tests pin it.

## Event.Expired(now Time) bool

Report whether ProcessedAt is nil and ExpiredAt is not after now.
