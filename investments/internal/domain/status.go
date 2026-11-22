package domain

import (
	"fmt"
	"time"
)

// InstrumentStatus is the price freshness of one held instrument.
type InstrumentStatus struct {
	InstrumentID    int
	Symbol          string
	Exchange        string
	LatestPriceDate *Date  // nil when the instrument has no stored price
	PriceIssue      string // empty when fresh, otherwise PriceIssueMissing or PriceIssueStale
}

// Status is the health view of the data. The raw log stays available through ListUpdates.
type Status struct {
	Date               Date // today in the market timezone
	Healthy            bool
	Reasons            []string // one per problem, empty when healthy
	Instruments        []InstrumentStatus
	LatestPriceDate    *Date // newest price date across all instruments, nil when none
	LatestSnapshotDate *Date // newest price date covered by a completed run, nil when none
	SnapshotMissing    bool  // LatestPriceDate has no completed snapshot
	NeedsAttention     []Update
	GivenUpEvents      []Event // outbox events given up on and not yet expired, newest first
	ExpiredEvents      []Event // outbox events not fully handled and expired; never affect health
}

// Summarize fills Reasons and Healthy from the other fields.
func (s *Status) Summarize() {
	s.Reasons = []string{}

	for _, instrument := range s.Instruments {
		if instrument.PriceIssue == "" {
			continue
		}

		reason := fmt.Sprintf("%s price %s", instrument.Symbol, instrument.PriceIssue)
		if instrument.LatestPriceDate != nil {
			reason += " (latest " + instrument.LatestPriceDate.Format(time.DateOnly) + ")"
		}

		s.Reasons = append(s.Reasons, reason)
	}

	if s.SnapshotMissing && s.LatestPriceDate != nil {
		s.Reasons = append(s.Reasons, "no completed snapshot for "+s.LatestPriceDate.Format(time.DateOnly))
	}

	if len(s.NeedsAttention) > 0 {
		s.Reasons = append(s.Reasons, fmt.Sprintf("%d failed or incomplete runs not yet superseded", len(s.NeedsAttention)))
	}

	if len(s.GivenUpEvents) > 0 {
		s.Reasons = append(s.Reasons, fmt.Sprintf("%d events given up on after repeated failures", len(s.GivenUpEvents)))
	}

	s.Healthy = len(s.Reasons) == 0
}
