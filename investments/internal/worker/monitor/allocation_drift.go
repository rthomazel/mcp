package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

const (
	groupsKey           = "groups"
	groupNameKey        = "name"
	groupInstrumentsKey = "instruments"
	groupMinKey         = "min_pct"
	groupMaxKey         = "max_pct"
	maxPercentBand      = "100"
)

// AllocationDrift watches groups of instruments leaving their allocation band.
type AllocationDrift struct {
	store  domain.GenericStore
	events domain.EventStore
}

// NewAllocationDrift builds the monitor.
func NewAllocationDrift(store domain.GenericStore, events domain.EventStore) *AllocationDrift {
	return &AllocationDrift{store: store, events: events}
}

// Name is the event type the monitor records without its notification prefix.
func (m *AllocationDrift) Name() string {
	return strings.TrimPrefix(domain.Events.AllocationDrift, domain.NotificationPrefix)
}

// ValidateParams requires a valid list of groups.
func (m *AllocationDrift) ValidateParams(params Params) error {
	_, err := allocationGroupsFrom(params)

	return err
}

// Run records one event per group outside its band, once for each stay.
func (m *AllocationDrift) Run(ctx context.Context, params Params) error {
	groups, err := allocationGroupsFrom(params)
	if err != nil {
		return err
	}

	drift, err := m.store.GetAllocationDrift(ctx, groups)
	if err != nil {
		return fmt.Errorf("reading allocation drift: %w", err)
	}

	if drift == nil {
		slog.InfoContext(ctx, "allocation monitor: no securities to judge")

		return nil
	}

	for _, ticker := range drift.Unregistered {
		slog.WarnContext(ctx, "allocation monitor: ticker is not a registered instrument", "ticker", ticker)
	}

	bands := make(map[string]domain.AllocationGroup, len(groups))
	for _, group := range groups {
		bands[group.Name] = group
	}

	var recorded int

	for _, group := range drift.Groups {
		if group.Side == "" {
			continue
		}

		created, err := m.record(ctx, drift.Date, bands[group.Name], group)
		if err != nil {
			return err
		}

		if created {
			recorded++
		}
	}

	slog.InfoContext(ctx, "allocation monitor: judged groups", "groups", len(groups), "recorded", recorded)

	return nil
}

func (m *AllocationDrift) record(ctx context.Context, date domain.Date, band domain.AllocationGroup, group domain.GroupAllocation) (bool, error) {
	data, err := json.Marshal(domain.AllocationDriftData{
		Group: group.Name, Side: group.Side, Date: date, StayStart: *group.StayStart,
		AllocationPercent: group.AllocationPercent, MinPercent: band.MinPercent, MaxPercent: band.MaxPercent, Members: group.Members,
	})
	if err != nil {
		return false, fmt.Errorf("encoding event data: %w", err)
	}

	subject := strings.Join([]string{group.Name, group.Side, group.StayStart.Format(time.DateOnly)}, "@")

	created, err := m.events.Record(ctx, domain.DraftEvent{Type: domain.Events.AllocationDrift, Subject: &subject, Data: data})
	if err != nil {
		return false, fmt.Errorf("recording event: %w", err)
	}

	return created, nil
}

func allocationGroupsFrom(params Params) ([]domain.AllocationGroup, error) {
	if err := params.Unknown(groupsKey); err != nil {
		return nil, err
	}

	entries, ok, err := params.Mappings(groupsKey)
	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, fmt.Errorf("setting %q is required", groupsKey)
	}

	var (
		groups  []domain.AllocationGroup
		names   = map[string]bool{}
		tickers = map[string]string{}
	)

	for i, entry := range entries {
		group, err := allocationGroupFrom(entry)
		if err != nil {
			return nil, fmt.Errorf("group %d: %w", i+1, err)
		}

		if names[group.Name] {
			return nil, fmt.Errorf("group %d: the name %q is used by another group", i+1, group.Name)
		}

		names[group.Name] = true

		for _, ticker := range group.Tickers {
			if other, taken := tickers[ticker]; taken {
				return nil, fmt.Errorf("group %q: %s is also in group %q, an instrument belongs to one group at most", group.Name, ticker, other)
			}

			tickers[ticker] = group.Name
		}

		groups = append(groups, group)
	}

	return groups, nil
}

func allocationGroupFrom(entry Params) (domain.AllocationGroup, error) {
	var group domain.AllocationGroup

	if err := entry.Unknown(groupNameKey, groupInstrumentsKey, groupMinKey, groupMaxKey); err != nil {
		return group, err
	}

	name, ok, err := entry.Text(groupNameKey)
	if err != nil {
		return group, err
	}

	if group.Name = strings.TrimSpace(name); !ok || group.Name == "" {
		return group, fmt.Errorf("setting %q is required and may not be empty", groupNameKey)
	}

	selection, ok, err := entry.Instruments(groupInstrumentsKey)
	if err != nil {
		return group, err
	}

	if !ok || selection.Held {
		return group, fmt.Errorf("setting %q must be a list of tickers", groupInstrumentsKey)
	}

	group.Tickers = selection.Tickers

	if group.MinPercent, err = percentBound(entry, groupMinKey); err != nil {
		return group, err
	}

	if group.MaxPercent, err = percentBound(entry, groupMaxKey); err != nil {
		return group, err
	}

	switch {
	case group.MinPercent == nil && group.MaxPercent == nil:
		return group, fmt.Errorf("at least one of %q and %q is required", groupMinKey, groupMaxKey)
	case group.MinPercent != nil && group.MaxPercent != nil && group.MinPercent.Compare(*group.MaxPercent) > 0:
		return group, fmt.Errorf("%q may not exceed %q", groupMinKey, groupMaxKey)
	}

	return group, nil
}

// percentBound reads an optional bound, a decimal from 0 to 100.
func percentBound(entry Params, key string) (*domain.Decimal, error) {
	bound, ok, err := entry.Decimal(key)
	if err != nil || !ok {
		return nil, err
	}

	if bound.Sign() < 0 || bound.Compare(maxPercentBand) > 0 {
		return nil, fmt.Errorf("setting %q must be between 0 and %s, got %q", key, maxPercentBand, bound)
	}

	return &bound, nil
}
