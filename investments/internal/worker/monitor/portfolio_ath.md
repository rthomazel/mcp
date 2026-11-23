# Constants

portfolioATHAt = 07:00 in the market timezone.
intervalKey = "interval", the one setting.
athDefaultInterval = "15d", the default interval.

# Types

## PortfolioATH

1. store domain.GenericStore, for the high state
2. events domain.EventStore, where the events are recorded and the last one is read

The monitor for the securities value reaching a new all-time high, at most once in each interval.

# Functions

## NewPortfolioATH(store domain.GenericStore, events domain.EventStore) *PortfolioATH

Build the monitor.

## PortfolioATH.Name() string

Return domain.Events.PortfolioATH without domain.NotificationPrefix, portfolio.ath.

## PortfolioATH.At() ClockTime

Return portfolioATHAt.

## PortfolioATH.ValidateParams(params Params) error

1. Reject any setting other than intervalKey with Params.Unknown.
2. Read the interval with Params.DurationOr and athDefaultInterval.
3. Return nil.

#### Errors

- **2.** return the duration error naming the setting and the text.

## PortfolioATH.Run(ctx Context, params Params) error

1. Read the interval as in ValidateParams.
2. Read the state through store.GetHighState.
3. if there is no state or it is not a new high, log at info level and return nil.
4. Read the date of the last portfolio ATH event with lastEventDate.
5. if withinInterval holds for it, log at info level and return nil.
6. Record a portfolio ATH event through events.Record, with the date formatted YYYY-MM-DD as its subject and a PortfolioATHData built from the state and the last date as its payload.
7. Return nil.

#### Errors

- **1.** if reading fails, return the error.

---

- **2.** if reading fails, return the wrapped error.

---

- **4.** return the error of lastEventDate.

---

- **6.** if encoding or recording fails, return the wrapped error.

A date is judged once because the date is the subject, so repeating the run, or a weekend, records nothing. A new high the interval suppresses is not announced later: only the date that is the new high when judged can be. State has Previous whenever it is a new high.
