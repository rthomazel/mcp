---
id: 2026-11-13-investments-mcp
type: spec
summary: Personal investment monitoring MCP server with ledger-based positions, daily prices and valuation snapshots.
author: Thom
created: 2026-11-13
agents: claude
---

# Investments MCP

## Status

Draft v1, design agreed with Thom in a prior session, restructured to SDD format.
Plan stages 1 to 3 (foundations, ledger, positions and valuation) are implemented.
The market data provider is selected and stage 4 is being modelled; the later stages are not.
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
- Manually initiated weekly investment reviews.

## Non-goals

- Brazilian assets and any non-USD instrument; FX conversion.
- Brokerage or Interactive Brokers integration; automated trading.
- Portfolio-level return calculations.
- Tax-lot accounting and cost basis.
- Automatic corporate-action ingestion.
- Fundamentals, news and broad market research. A separate market-trends
  workflow covers these.
- Frontend or dashboard.
- Scheduled LLM reviews and notifications.
- A reusable investment-review skill. It may follow later.

## Concepts

The system keeps five kinds of records.

**Instruments** are the registry of tradable things: ticker, listing exchange,
name, quoted currency (USD only), and a type such as ETF or stock. An instrument is
unique by its ticker and listing exchange.

**Transactions** form an append-only-by-default ledger of ownership changes and
financial activity. Each entry belongs to one instrument and records an event
type, the change in share quantity, the cash impact in USD, when it was executed,
the date it economically applies to, and an optional note. Quantity and cash are
independent of each other.

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
- Chronological order is defined by effective date. An event executed outside
  market hours takes the effective date of the next trading session. Events sharing
  an effective date are ordered deterministically by execution time, then by identity.
- Event types are constrained by the database.
- The ledger is not a complete brokerage cash reconciliation and not tax-lot accounting.

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

### Cash

Cash movements are recorded in the ledger but excluded from valuation, allocation
and snapshots. Valuation represents securities only, and results name it securities
value in USD so it is not mistaken for brokerage account equity. Dividends and
withholding taxes are entered through the ledger; there is no distributions table
and no automatic dividend ingestion.

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

Price refresh. A refresh fetches, for each instrument, everything from the latest
stored date onward, inclusive, and inserts or updates the records. The latest stored
day is fetched again because the provider revises a day's figures after first
publishing it. A refresh never reaches further back than the latest stored date;
correcting older prices is a deliberate, separate action. When the provider throttles
requests, the refresh stops, reports which instruments were not refreshed, and
can be run again later. One instrument failing does not stop the others.

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
read historical snapshots; trigger a market data refresh; and rebuild snapshots
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
- Valuation basis is securities market value only, in USD.
- Corrections are administrative, not an agent feature.
- Extensions such as FX, cash reconciliation, cost basis, portfolio returns and
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

## Open questions

1. **Daily schedule.** Set in the snapshot stage from the provider's publishing
   times: US equities about 5:30 PM Eastern with corrections possible until 8 PM,
   mutual funds after midnight Eastern.

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

## Refs

- Related: mcp/bench/spc for the spec conventions used by sibling servers.
- Related: interface doc/skills/tiingo-api, how the provider API behaves, validated live.
