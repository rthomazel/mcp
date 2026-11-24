package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"slices"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

const (
	// resolveTickersSQL takes $1, the tickers, and lists the registered instruments with those symbols.
	resolveTickersSQL = `SELECT symbol, id FROM instruments WHERE symbol = ANY($1::text[]) ORDER BY symbol, id`

	// allocationMembersCTE takes $1 the group names with $2 and $3 their bounds as text, an empty text meaning
	// none, and $4 the group of each member with $5 the member's instrument id.
	allocationMembersCTE = `
bands AS (
    SELECT name, NULLIF(min_text, '')::numeric AS min_pct, NULLIF(max_text, '')::numeric AS max_pct
    FROM unnest($1::text[], $2::text[], $3::text[]) AS b(name, min_text, max_text)
), members AS (
    SELECT group_name, instrument_id FROM unnest($4::text[], $5::int[]) AS m(group_name, instrument_id)
)`

	// allocationDriftSQL judges each group on every date of the securities series. A group is outside the band
	// by cross multiplication, so no division rounds it, and never on a date without securities. The stay start
	// is the date after the most recent date on a different side, else the first date of the series.
	allocationDriftSQL = `WITH ` + dailySeriesSQL + `, ` + allocationMembersCTE + `, group_daily AS (
    SELECT b.name, b.min_pct, b.max_pct, d.date, d.securities AS total, COALESCE(SUM(s.market_value_usd), 0) AS value
    FROM bands b CROSS JOIN daily d
    LEFT JOIN members m ON m.group_name = b.name
    LEFT JOIN snapshots s ON s.date = d.date AND s.instrument_id = m.instrument_id
    GROUP BY b.name, b.min_pct, b.max_pct, d.date, d.securities
), judged AS (
    SELECT *, CASE
        WHEN total > 0 AND max_pct IS NOT NULL AND value * 100 > max_pct * total THEN 'above'
        WHEN total > 0 AND min_pct IS NOT NULL AND value * 100 < min_pct * total THEN 'below'
    END AS side
    FROM group_daily
), latest AS (
    SELECT MAX(date) AS date FROM daily
)
SELECT latest.date, j.name, trim_scale(COALESCE(j.value * 100 / NULLIF(j.total, 0), 0))::text, j.side,
       CASE WHEN j.side IS NULL THEN NULL ELSE COALESCE(
           (SELECT MIN(x.date) FROM judged x WHERE x.name = j.name AND x.date > (
               SELECT MAX(y.date) FROM judged y WHERE y.name = j.name AND y.side IS DISTINCT FROM j.side)),
           (SELECT MIN(date) FROM daily)) END,
       j.total > 0
FROM judged j JOIN latest ON j.date = latest.date`

	// allocationMemberSharesSQL takes the same parameters and lists each registered member's share on the latest date.
	allocationMemberSharesSQL = `WITH ` + dailySeriesSQL + `, ` + allocationMembersCTE + `, latest AS (
    SELECT date, securities AS total FROM daily ORDER BY date DESC LIMIT 1
)
SELECT m.group_name, i.symbol, trim_scale(COALESCE(s.market_value_usd, 0) * 100 / NULLIF(latest.total, 0))::text
FROM members m
JOIN instruments i ON i.id = m.instrument_id
CROSS JOIN latest
LEFT JOIN snapshots s ON s.date = latest.date AND s.instrument_id = m.instrument_id
ORDER BY m.group_name, i.symbol`
)

type allocationArgs struct {
	names, mins, maxs, memberGroups []string
	memberIDs                       []int32
}

// GetAllocationDrift judges every group against its band on the latest date.
func (s *Monitors) GetAllocationDrift(ctx context.Context, groups []domain.AllocationGroup) (*domain.AllocationDrift, error) {
	args, unregistered, err := s.allocationArgs(ctx, groups)
	if err != nil {
		return nil, err
	}

	byName := make(map[string]*domain.GroupAllocation, len(groups))

	var (
		latest        sql.NullTime
		hasSecurities = true
	)

	rows, err := s.db.QueryxContext(ctx, allocationDriftSQL, args.names, args.mins, args.maxs, args.memberGroups, args.memberIDs)
	if err != nil {
		return nil, fmt.Errorf("reading allocation drift: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var (
			group     domain.GroupAllocation
			side      sql.NullString
			stayStart sql.NullTime
			held      bool
		)

		if err := rows.Scan(&latest, &group.Name, &group.AllocationPercent, &side, &stayStart, &held); err != nil {
			return nil, fmt.Errorf("scanning allocation drift: %w", err)
		}

		hasSecurities = hasSecurities && held
		group.Side = side.String

		if stayStart.Valid {
			group.StayStart = &stayStart.Time
		}

		group.Members = []domain.MemberAllocation{}
		byName[group.Name] = &group
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading allocation drift: %w", err)
	}

	if len(byName) == 0 || !hasSecurities {
		return nil, nil //nolint:nilnil // no snapshots, or no securities on the latest date
	}

	if err := s.addMemberShares(ctx, args, byName); err != nil {
		return nil, err
	}

	drift := &domain.AllocationDrift{Date: latest.Time, Unregistered: unregistered}
	for _, group := range groups {
		drift.Groups = append(drift.Groups, *byName[group.Name])
	}

	return drift, nil
}

// allocationArgs resolves the tickers of the groups to instrument ids. A ticker on several
// instruments is an error; an unregistered one is returned and has no member row.
func (s *Store) allocationArgs(ctx context.Context, groups []domain.AllocationGroup) (allocationArgs, []string, error) {
	var tickers []string

	for _, group := range groups {
		tickers = append(tickers, group.Tickers...)
	}

	ids, err := s.resolveTickers(ctx, tickers)
	if err != nil {
		return allocationArgs{}, nil, err
	}

	args := allocationArgs{}
	registered := map[string]bool{}

	for _, group := range groups {
		args.names = append(args.names, group.Name)
		args.mins = append(args.mins, boundText(group.MinPercent))
		args.maxs = append(args.maxs, boundText(group.MaxPercent))

		for _, ticker := range group.Tickers {
			if id, ok := ids[ticker]; ok {
				args.memberGroups = append(args.memberGroups, group.Name)
				args.memberIDs = append(args.memberIDs, int32(id)) //nolint:gosec // instrument ids are serial integers
				registered[ticker] = true
			}
		}
	}

	var unregistered []string

	for _, ticker := range tickers {
		if !registered[ticker] && !slices.Contains(unregistered, ticker) {
			unregistered = append(unregistered, ticker)
		}
	}

	return args, unregistered, nil
}

// resolveTickers maps each registered ticker to its instrument id. A ticker that
// matches more than one instrument is domain.ErrInvalidInput.
func (s *Store) resolveTickers(ctx context.Context, tickers []string) (map[string]int, error) {
	rows, err := s.db.QueryxContext(ctx, resolveTickersSQL, tickers)
	if err != nil {
		return nil, fmt.Errorf("resolving tickers: %w", err)
	}
	defer func() { _ = rows.Close() }()

	ids := map[string]int{}

	for rows.Next() {
		var (
			symbol string
			id     int
		)

		if err := rows.Scan(&symbol, &id); err != nil {
			return nil, fmt.Errorf("scanning ticker: %w", err)
		}

		if _, taken := ids[symbol]; taken {
			return nil, fmt.Errorf("%w: the ticker %s matches more than one instrument", domain.ErrInvalidInput, symbol)
		}

		ids[symbol] = id
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("resolving tickers: %w", err)
	}

	return ids, nil
}

func (s *Store) addMemberShares(ctx context.Context, args allocationArgs, byName map[string]*domain.GroupAllocation) error {
	rows, err := s.db.QueryxContext(ctx, allocationMemberSharesSQL, args.names, args.mins, args.maxs, args.memberGroups, args.memberIDs)
	if err != nil {
		return fmt.Errorf("reading member allocations: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var (
			name   string
			member domain.MemberAllocation
		)

		if err := rows.Scan(&name, &member.Symbol, &member.AllocationPercent); err != nil {
			return fmt.Errorf("scanning member allocation: %w", err)
		}

		group, ok := byName[name]
		if !ok {
			return fmt.Errorf("member allocation for unknown group %q", name)
		}

		group.Members = append(group.Members, member)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("reading member allocations: %w", err)
	}

	return nil
}

func boundText(bound *domain.Decimal) string {
	if bound == nil {
		return ""
	}

	return string(*bound)
}
