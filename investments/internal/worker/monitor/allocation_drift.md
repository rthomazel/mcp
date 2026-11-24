# Constants

groupsKey = "groups", the one setting.
groupNameKey = "name", groupInstrumentsKey = "instruments", groupMinKey = "min_pct", groupMaxKey = "max_pct", the keys of a group.
maxPercentBand = "100", the largest bound.

# Types

## AllocationDrift

1. store domain.MonitorStore, for the allocation of each group
2. events domain.EventStore, where the events are recorded

The monitor for groups of instruments leaving their allocation band.

# Functions

## NewAllocationDrift(store domain.MonitorStore, events domain.EventStore) *AllocationDrift

Build the monitor.

## AllocationDrift.Name() string

Return domain.Events.AllocationDrift without domain.NotificationPrefix, allocation.drift.

## AllocationDrift.ValidateParams(params Params) error

Return the error of allocationGroupsFrom, or nil.

## AllocationDrift.Run(ctx Context, params Params) error

1. Read the groups with allocationGroupsFrom.
2. Read the allocations through store.GetAllocationDrift.
3. if there is none, log at info level and return nil.
4. Log each unregistered ticker at warning level.
5. For each group outside its band, record an allocation drift event with record, in group order.
6. Log at info level how many groups were judged and how many events were recorded.
7. Return nil.

#### Errors

- **1.** return the error.

---

- **2.** if reading fails, return the wrapped error; an ambiguous ticker fails the run so the operator resolves it.

---

- **5.** if recording fails, return the error; the groups not yet recorded are recorded by the next run.

Recording is idempotent per subject, so a later run in the same stay records nothing, and a stay that ended and started again has a new start and so a new subject. No state beyond the outbox.

## AllocationDrift.record(ctx Context, date Date, band domain.AllocationGroup, group domain.GroupAllocation) (bool, error)

Record the event through events.Record with the group name, the side and the stay start as YYYY-MM-DD joined by at signs as its subject, and an AllocationDriftData built from the group, the date and the configured bounds as its payload. Return whether it was created, or the wrapped error.

## allocationGroupsFrom(params Params) ([]domain.AllocationGroup, error)

1. Reject any setting other than groupsKey with Params.Unknown.
2. Read groups with Params.Mappings; it is required.
3. Build each group with allocationGroupFrom, naming its position in any error.
4. Reject a name used by another group, and a ticker that is in more than one group.
5. Return the groups in order.

#### Errors

- **2.** a missing, empty or malformed list is an error naming the setting.

---

- **3.** return the error wrapped with the group's position.

---

- **4.** return an error naming the name or the ticker and the group that has it.

## allocationGroupFrom(entry Params) (domain.AllocationGroup, error)

1. Reject any key other than the four group keys.
2. Read the name with Params.Text; it is required and not empty once trimmed.
3. Read the instruments with Params.Instruments; a list of tickers is required and held is not accepted.
4. Read the bounds with percentBound.
5. Require at least one bound, and a minimum that does not exceed the maximum by Decimal.Compare.
6. Return the group.

#### Errors

- **1.** return an error naming the key.

---

- **2.** return an error naming the setting.

---

- **3.** a missing instruments setting, the word held, or a list with an empty or repeated ticker is an error.

---

- **5.** return an error naming the settings.

## percentBound(entry Params, key string) (*domain.Decimal, error)

Read the optional bound with Params.Decimal and require 0 to maxPercentBand inclusive with Decimal.Compare; nil when absent. Bounds are quoted text, like every exact number.
