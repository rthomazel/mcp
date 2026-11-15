---
id: 2026-11-13-investments-mcp
type: spec
summary: Personal investment monitoring MCP server with ledger-based positions, daily prices and valuation snapshots.
author: Thom
created: 2026-11-13
updated: 2026-11-15
agents: claude
---

# Investments MCP

## Status

Draft v1, design agreed with Thom in a prior session, restructured to SDD format.
Plan stages 1 to 3 (foundations, ledger, positions and valuation) are implemented.
The v1 release is deployed and in operator testing. Operator feedback drives a follow-up round, specified in
the sections Cash and account equity, Position P&L, Status and Value history. The round is delivered in this order:
cash, then P&L, then value history, then status; the last two are independent of the first two.
Remaining open questions are at the end.

## Intent

A lightweight personal investment monitoring system. It keeps reliable historical
records of what is owned, what financial events happened, what the market prices
were, and what the portfolio was worth each day. An existing agent, used from the
LibreChat frontend and reaching the server through the LiteLLM gateway over MCP,
answers portfolio questions and supports weekly investment reviews that the
operator starts manually.

The goal is trustworthy portfolio data and visibility into changes in allocation,
concentration and risk. It is not a brokerage integration or an accounting platform.

The MCP server is the application backend. It owns database access, calculations,
market data ingestion and the market data provider credential. There is no
separate backend API service. It is deployed on the existing LGA VPS and uses its
own database on a shared PostgreSQL server (LGA Postgres).

## Scope

Included:

- An international portfolio of USD-denominated instruments only.
- Manually maintained instruments and financial events.
- Daily historical market prices.
- Daily portfolio valuation snapshots, one record per instrument per day.
- Current and historical position reconstruction.
- Securities valuation and allocation.
- An explicit cash balance, account equity and buying power.
- Position P&L under average cost, with realized and unrealized parts.
- A freshness and health status view and a portfolio value history.
- Manually initiated weekly investment reviews.

## Non-goals

- Brazilian assets and any non-USD instrument; FX conversion.
- Brokerage or Interactive Brokers integration; automated trading.
- Portfolio-level return calculations: time-weighted or money-weighted performance. Position P&L and the value history are not returns.
- Tax-lot accounting. Cost basis is average cost only.
- Margin, multiple accounts and cash-like instruments such as T-bills.
- Automatic corporate-action ingestion.
- Fundamentals, news and broad market research. A separate market-trends
  workflow covers these.
- Frontend or dashboard.
- Scheduled LLM reviews and notifications.
- A reusable investment-review skill. It may follow later.

## Concepts

The system keeps six kinds of records.

**Instruments** are the registry of tradable things: ticker, listing exchange,
name, quoted currency (USD only), and a type such as ETF or stock. An instrument is
unique by its ticker and listing exchange.

**Transactions** form an append-only-by-default ledger of ownership changes and
financial activity. Each entry belongs to one instrument and records an event
type, the change in share quantity, the cash impact in USD, when it was executed,
the date it economically applies to, and an optional note. Quantity and cash are
independent of each other. A buy or sell may also record the fee it contained.

**Cash transactions** are account-level cash events that belong to no instrument: an
opening balance, deposits, withdrawals, interest and account fees. They are distinct from
transactions, the instrument ledger. Together with the cash impact of transactions they
make up the cash ledger.

**Cash snapshots** are daily cash balances, one record per snapshot date, so account
equity has a history that is preserved like the securities snapshots.

**Prices** are daily historical market prices per instrument: open, high, low,
close, adjusted close and volume.

**Snapshots** are daily portfolio valuations, one record per instrument per date,
holding the quantity owned, the closing price used, the date of that price, and the
resulting market value. Portfolio totals and allocation come from aggregating the
rows for a date.

**Updates** are the log of ingestion runs, snapshot rebuilds and their outcomes,
used to see failures, freshness and history.

## Behavior

### Ledger events

Six event types exist, each with a sign convention for quantity and cash.

- Buy: quantity increases, cash is negative.
- Sell: quantity decreases, cash is positive.
- Dividend: no quantity change, cash is positive.
- Tax: no quantity change, cash is negative.
- Split: a quantity adjustment with no cash. Used for splits and reverse splits.
- Adjustment: a quantity adjustment with optional cash. Used for reinvested
  dividends and manual fixes.

Examples: buying 10 shares at 500 dollars records plus 10 shares and minus 5000
cash; selling 3 at 520 records minus 3 and plus 1560; a 50 dollar dividend records
no quantity and plus 50; 7.50 of withholding tax records no quantity and minus 7.50;
reinvesting 50 dollars into 0.1 shares records plus 0.1 and minus 50; a 2-for-1
split on 100 shares records plus 100 and no cash.

Rules:

- Entries are normally retained and not modified. Partial sales are negative
  quantity changes. Repeated purchases of one instrument are independent events.
- A buy or sell may carry a fee, a non-negative amount that is part of the recorded cash amount, never in addition to it. See Position P&L.
- Chronological order is defined by effective date. An event executed outside
  market hours takes the effective date of the next trading session. Events sharing
  an effective date are ordered deterministically by execution time, then by identity.
- Event types are constrained by the database.
- Cash is complete only from the opening balance onward; see Cash and account equity.

### Long-only integrity

Short positions are unsupported. A transaction that would drive ownership of an
instrument negative at any point in the chronological ledger is rejected before it
is committed. Order is the chronological order defined above. The check covers the
whole timeline, not only the final balance, so
a backdated sale cannot create a negative balance in between later events.
Negative balances that already exist are reported as data-integrity errors and are
never silently omitted from results. Concurrent writes touching the same instrument
must serialize so two sales cannot both pass validation against the same balance.

### Corrections

There is no reversal or correction-chain mechanism. For an erroneous entry, an
administrator deletes the row directly in the database and inserts the corrected
one. The agent-facing interface never deletes transactions. A correction that
touches historical dates requires an explicit snapshot rebuild from the affected
effective date onward, which the agent can trigger.

### Positions and valuation

Current positions are derived from the ledger by summing quantity changes per
instrument. A historical position uses the same sum restricted to entries whose
effective date is on or before the requested date. Only instruments with a positive
resulting quantity are active positions.

An instrument's value is its quantity times its closing price. Securities value is
the sum over instruments, and an instrument's allocation is its value divided by
securities value. These calculations are deterministic and done by the backend,
never by the language model; the model interprets the results.

Valuation always uses the real closing price. Adjusted close is retained for future
instrument-level total-return analysis and is never substituted for valuing actual
holdings.

### Cash and account equity

Cash is modelled explicitly and never treated as a security. Cash transactions are a
separate record of five kinds, each with a sign convention: an opening balance
(any sign, at most one), deposit and interest (positive), withdrawal and fee (negative).
Dividends, taxes and trades keep entering through the transaction ledger, and their
cash impact counts toward cash from there. Nothing is entered twice.

The cash balance on a date is the sum of the opening balance, all cash transactions
and all transaction cash amounts on or after the opening balance date and on or
before the date. Entries dated before the opening balance are not part of cash. With no opening balance, or a date
before it, cash is reported as not initialized and no balance, equity or buying power is
shown; the agent is told to record an opening balance. The current balance and the balance on any
requested date are computed from the cash ledger.

Cash snapshots preserve history. The same unit of work that writes a date's securities
snapshot also writes that date's cash snapshot, the balance computed as above, whenever cash is
initialized on that date. A date with no active positions still gets a cash snapshot, so
a portfolio that is entirely cash keeps an equity history. A rebuild replaces cash snapshots with the
securities snapshots over the same range, atomically, and the snapshot run's completion marker covers both. A
cash snapshot is never merged into the securities snapshot rows: those stay one row per instrument and
allocation stays over securities only.

Cash-like holdings such as T-bills are not cash. They are out of scope for now; when added they
will be instruments with their own valuation in the securities snapshots, and cash snapshots stay unchanged.

Account equity is securities value plus cash. It is shown beside securities value and
never replaces it, and is withheld when the portfolio is incomplete or cash is not
initialized. Buying power is the cash balance, floored at zero; there is no margin.

A balance that is negative at any date is not rejected, because transactions were
recorded before cash existed. It is reported as a data-integrity error, like a
negative position, and never silently omitted.

Valuation, allocation and snapshots still represent securities only.
Dividends and withholding taxes are entered through the ledger; there is no
distributions table and no automatic dividend ingestion.

### Position P&L

Cost basis is average cost, computed by the backend over the ledger in chronological order,
per instrument. There are no tax lots.

- Buy: the cash paid, fee included, is added to cost; quantity increases.
- Sell: realized P&L is the cash received, net of fee, minus the average cost of the shares sold.
  Cost falls by that average cost; the average cost per share is unchanged.
- Split: quantity changes, cost is unchanged, so the average cost per share adjusts.
- Adjustment: a quantity increase with cash paid adds that cash to cost, which is how a
  reinvested dividend enters basis; a quantity increase with no cash adds shares at zero cost; a
  quantity decrease realizes like a sell using its cash, if any.
- Dividends and taxes do not touch cost basis.

For each position as of a date the backend reports quantity, total cost, average cost, market value,
unrealized P&L in dollars and percent of cost, and cumulative realized P&L, dividends received,
tax withheld and fees paid, each separately. Fees are already inside cost and sale proceeds,
so they are reported for information and are never subtracted again. An optional total-return view sums
unrealized and realized P&L plus dividends minus taxes. It is a position
total, not a time-weighted or money-weighted return. A position that is fully sold stays
visible with its realized P&L, dividends and tax. Positions with
no usable price withhold market value and unrealized P&L, and portfolio totals are withheld likewise, as in
valuation. Realized P&L is cumulative since the first ledger entry up to the date.

### Corporate actions

Splits and reverse splits are entered manually as split events that adjust share
quantities. Stored market prices are not rewritten. Dividends and their withholding
taxes are separate ledger events. Adjusted closes do not by themselves establish
portfolio performance.

### Market data

The provider sits behind a provider-independent boundary so it can be replaced
without changing the stored data or the agent-facing contract. The selected
provider is Tiingo, on its free plan, whose terms allow this personal, internal
use. It was chosen over Alpha Vantage and Stooq for daily raw and adjusted prices,
decades of history, dividend and split data, usable free limits and clear
licensing. Only the provider's own data enters storage; the agent-facing contract
never exposes provider-specific shapes.

No custom market-calendar service exists. The provider identifies a security by its
ticker alone, so the instrument's exchange is descriptive and is never sent to it.
The backend asks the provider for the latest completed daily price and relies
on the provider for sessions, weekends, holidays and delays. Missing or unexpectedly
old prices are flagged. A price is unexpectedly old when it is five or more days
older than the run date. Only US-listed securities the provider covers are supported.
Mutual funds publish a day later than exchange-traded securities and carry no
volume, so their latest price can legitimately lag.

Price backfill. Registering an instrument first asks the provider for its full
available history. If the provider does not know the ticker, the registration is
rejected and nothing is stored, so an instrument that can never be priced is
never created. Otherwise the instrument is registered and the history stored. If
storing the history fails, the instrument stays registered with no prices, and the
next refresh fetches its full history, so the gap heals without intervention.

Price refresh. A refresh covers the instruments with a positive quantity; a sold instrument is no longer refreshed, which saves provider requests, and its prices stop where they were. A refresh fetches, for each of them, everything from the latest
stored date onward, inclusive, and inserts or updates the records. The latest stored
day is fetched again because the provider revises a day's figures after first
publishing it. A refresh never reaches further back than the latest stored date;
correcting older prices is a deliberate, separate action. When the provider throttles
requests, the refresh stops, reports which instruments were not refreshed, and
can be run again later. One instrument failing does not stop the others. An instrument that becomes held again, or is bought for the first time, catches up from its latest stored date through today, and recording the buy that opens the position triggers that catch-up immediately instead of waiting for the next daily run.

### Daily ingestion and snapshots

A background job runs independently of the language model and:

1. fetches the latest available market prices;
2. inserts or updates historical price records;
3. reconstructs current positions from the ledger;
4. picks the applicable closing prices;
5. computes the daily securities valuation;
6. writes instrument-level snapshots; and
7. records the outcome in the updates log.

Snapshot cadence: a snapshot exists for a date only when new price data exists for
that date, so in practice for each day the market was open. Weekends and holidays
produce no snapshot. A snapshot's date and its price date therefore coincide for
normal runs.

Snapshot policy:

- Daily generation is idempotent. The marker for a completed date is a successful
  run recorded for it in the updates log, not the presence of snapshot rows, so a
  date with no active positions is also correctly treated as done. A retry for a
  completed date does nothing.
- Ingestion never silently overwrites existing snapshots.
- A missing price is never replaced with zero or a stale price. Missing or
  unexpectedly old prices are reported and the run is marked incomplete. A snapshot
  is incomplete if any active instrument lacks a usable price, and in that case no
  snapshot rows are written for the date. Incomplete runs are not a completed date
  and may be retried later.
- Corrected historical prices, backdated transactions and ledger corrections require
  an explicit rebuild from the affected date onward.
- A rebuild is atomic: the whole requested range is rebuilt or nothing changes.
- Every rebuild is logged in the updates log with its reason and date range.
- Historical snapshots preserve the portfolio as it was; they are not recalculated
  when holdings or prices change later.

The job runs as a background routine inside the server process. Its schedule
accounts for US market close times and daylight-saving changes, and for when the
provider can safely serve the day's data. No exchange-specific scheduler exists.

### Agent capabilities

Through MCP the agent can: register an instrument; list instruments; list ledger
events; record transactions of any supported type; reconstruct current or historical
positions; read current securities valuation and allocation; read historical prices;
read historical snapshots; read the updates log; record cash transactions and read the cash balance, account equity and buying power; read position P&L; read the status view and the value history; trigger a market data refresh; and rebuild snapshots
over a date range with a stated reason. Registering an instrument rejects any that
is not USD-quoted. Transaction deletion is not an agent capability. Exact tool
contracts are defined in the models.

Writes validate event types, amounts, instrument references and ledger consistency.
No separate confirmation or approval workflow exists at the server boundary.

### Access and security

- The service is not publicly exposed. It listens only on the LGA container
  network, which sits inside the Tailscale network.
- LiteLLM is the gateway. The server is registered in LiteLLM, which exposes it to
  LibreChat and enforces API-key authentication. Write capabilities, including
  rebuilds, are available only to authenticated clients.
- The market data credential is a server-side secret and never appears in MCP
  responses. Provider and gateway credentials must support rotation.
- Write activity and ingestion results are logged operationally.

### Status

A status view answers whether the data is healthy, beyond the raw updates log. It reports,
for each held instrument, its latest stored price date and whether it is missing or unexpectedly
old; the latest completed snapshot date and whether the latest price date has no completed
snapshot; and the failed or incomplete runs that still need attention. A run needs
attention until a later completed run of the same kind, for a snapshot run the same date,
supersedes it. The view gives one overall verdict, healthy or needs attention, with reasons.
The raw updates log stays available.

### Value history

A value history query returns the securities value over a date range at daily, weekly or monthly
granularity. Each point is the value on the last snapshot date within its interval, so weekends and
holidays never appear. Dates without a completed snapshot are absent, never interpolated.
When a cash snapshot exists for the point, the point also carries cash and account equity beside it.

Summary measures cover the requested range, except the all-time high:

- latest value and date;
- all-time high and its date, over every stored snapshot since the earliest one, whatever the range;
  the earliest date wins a tie;
- dollar and percentage drawdown of the latest value from the all-time high;
- change over the range, dollar and percent, from the first point to the last.

The same measures are also reported on account equity, over the dates that have a cash snapshot, together with the net contributions in the range, deposits minus withdrawals, so a change in equity can be read against money added or removed.

All measures are changes in value, not investment return: a deposit
or a new purchase raises value without being a gain, and results say so.

### Weekly review

The operator starts a review by asking the agent to use this server. A review
typically retrieves current positions and valuation, compares allocation with
earlier snapshots, flags concentration and significant valuation changes, looks at
per-instrument price performance and unusual moves, and discusses possible actions.
Fundamentals, news, benchmarks and macro analysis belong to the separate
market-trends workflow.

## Deployment

LGA gets one PostgreSQL server, called LGA Postgres, hosting multiple databases.
The Investments MCP uses a dedicated investments database on it, with its own role
that has no access to other databases. Future systems get their own databases on
the same server rather than their own servers. This is the first database for the
operator's own services; the databases behind other LGA services belong to
third-party projects that deploy their own, and are out of scope. A read replica may be added later,
for example to serve the trading service; that is not part of v1.

LGA Postgres and the server are added to the LGA stack as compose services alongside
the existing MCP servers, with their secrets managed the way LGA manages the others. The server is then registered in LiteLLM so LibreChat
can use it. Database storage must persist across restarts and be
covered by a backup service that runs daily, per database. A backup that has
not succeeded recently must be visible, and restoring from it must be verified at
least once before the system is relied on. Staging and rollout order are in the plan.

## Decisions

- One Go service and five tables; no separate API backend.
- One shared PostgreSQL server on LGA with a database per system; this server owns
  only the investments database.
- PostgreSQL numeric types for all money and quantities.
- Ownership history comes from the ledger; portfolio history from daily
  instrument-level snapshots.
- Valuation basis is securities market value only, in USD; account equity is shown beside it, not merged.
- Cash is a separate explicit record of cash transactions, with an opening balance as its anchor and daily cash snapshots for history.
- Cost basis is average cost. Trade fees are a field on buys and sells, included in cost and proceeds.
- Value history and position P&L are not returns.
- Corrections are administrative, not an agent feature.
- Extensions such as FX, tax lots, portfolio returns, T-bills and
  automatic corporate actions are future work, added only when justified.

## Acceptance criteria

- A buy, sell, dividend, tax, split and adjustment can each be recorded and
  produce the expected position and cash effects.
- A sale exceeding the holding, or a backdated sale that would make any later point
  in the timeline negative, is rejected and nothing is written.
- Two concurrent sales of the same instrument cannot together exceed the holding.
- Positions for any past date match the ledger up to that effective date.
- Portfolio valuation uses real closes, excludes cash, and allocation sums to one.
- A daily run on a day with all prices available writes one snapshot per active
  instrument; a repeat run changes nothing. No snapshot is written for a weekend
  or holiday.
- A run on a date with no active positions is recorded complete and not re-run.
- A price five or more days old is flagged and the run recorded as incomplete.
- When one active instrument lacks a price for a date, no snapshot rows are written
  for that date.
- Registering an instrument with a non-USD currency is rejected; a duplicate
  ticker and exchange is rejected.
- An event executed after market close is ordered by the next session's effective date.
- A daily run with a missing price writes no zero or stale value and the run is
  recorded as incomplete.
- A rebuild over a date range is all-or-nothing and appears in the updates log with
  reason and range.
- The provider credential never appears in any response or log.
- The server is unreachable from outside the container and Tailscale networks, and
  through LiteLLM rejects requests without a valid key.
- A daily backup of the investments database runs, and a restore from it has been
  verified.
- Swapping the market data provider requires no change to stored data or agent contracts.
- Cash: with no opening balance, cash, equity and buying power are reported as not initialized. With one, the balance equals the opening balance plus cash transactions and transaction cash from its date, a trade's cash is counted once, and a negative balance is reported as an integrity error and never rejected.
- A snapshot run writes the cash snapshot with the securities snapshot when cash is initialized, including on a date with no active positions; a rebuild replaces both atomically; value history reports equity and net contributions from cash snapshots.
- Account equity equals securities value plus cash, is shown beside securities value, and is withheld when the portfolio is incomplete.
- Average cost: after two buys and a partial sell, realized P&L, remaining cost and average cost match hand-calculated values; a split changes quantity and average cost but not total cost and realizes nothing; a reinvested dividend adds to cost.
- A trade fee is within the recorded cash amount, counted once in cash, in cost on a buy and in net proceeds on a sell, and reported separately.
- A fully sold position shows its realized P&L; a position with no usable price withholds unrealized P&L; the total-return view equals unrealized plus realized plus dividends minus taxes.
- Status flags a held instrument with a missing or five-day-old price and a latest price date with no completed snapshot, lists unresolved failed or incomplete runs, and clears an item once a later run supersedes it.
- Value history returns one point per interval at the last snapshot date, skips non-snapshot dates, and the all-time high and drawdown cover every stored snapshot regardless of the range.

## Resolved

- Provider is Tiingo, selected through the market data spike. Reachability from the VPS was verified by the operator.
- Price backfill pulls the full available history when an instrument is registered; later refreshes fetch from the latest stored date inclusive.
- Registering instruments and rebuilding snapshots are agent capabilities.
- Chronological order follows effective date; after-hours events take the next session's date.
- Snapshots only for days with new price data.
- A snapshot date is all-or-nothing: if any active instrument lacks a usable price, nothing is written for that date, the run is recorded incomplete, and it is retried later.
- A completed run in the updates log is the idempotency marker.
- Instruments are unique by ticker and exchange.
- Unexpectedly old means five or more days.
- LiteLLM is the gateway and exposes the server to LibreChat.
- The daily job is a routine inside the server process.
- Daily schedule: the job runs at 8:30 PM and again at 6:00 AM Eastern, in the market timezone so daylight saving is followed. The evening run follows the US equity close, publication and corrections window; the morning run picks up mutual funds, which publish after midnight, so a date left incomplete in the evening completes the next morning.
- The daily job snapshots only the latest date that has price data. A date missed while the server was down is not back-filled automatically; the agent fills it with a rebuild. A rebuilt date counts as completed.
- Agents refer to instruments by symbol, with the exchange only when the symbol alone is ambiguous; internal ids are never exposed.
- Tool results are JSON with decimals as exact strings. Recording a transaction never rebuilds snapshots; when it makes snapshots stale, the result names the date to rebuild from.
- The agent can read the updates log, to check freshness and why a day has no snapshot.
- Follow-up round decisions (operator feedback, 2026-11-15): cash is a separate cash transactions record with daily cash snapshots; cost basis is average cost; trades carry an optional fee field; all four items are specified together, with cash then P&L implemented first.
- A snapshot requires every active instrument to have a price dated exactly on the snapshot date.

## Refs

- Related: mcp/bench/spc for the spec conventions used by sibling servers.
- Related: interface doc/skills/tiingo-api, how the provider API behaves, validated live.
