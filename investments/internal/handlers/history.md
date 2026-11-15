# Functions

## Handler.HandleGetValueHistory(ctx Context, req CallToolRequest) (*CallToolResult, error)

Tool get_value_history. Reports the securities value over time, with highs, drawdown and change. Results say they are changes in value, not investment return.

Description shown to the agent: Report portfolio value over a date range with one point per window of window_days days, from the stored daily snapshots; 1 is daily, 7 about weekly, 30 about monthly, and windows are counted from the range start. Each point is the value on the last snapshot date within its window, so weekends and holidays never appear and dates without a snapshot are absent, never interpolated; points carry cash and account equity when a cash snapshot exists for the date. The summary, for securities value and again for account equity when cash snapshots exist, gives the latest value, the all-time high and its date measured from the earliest snapshot through the range end, the drawdown of the latest value from that high, the change from the baseline, which is the last snapshot on or before the range start and is named in the result, the high and low within the range, and the maximum drawdown within the range with its peak and trough dates and whether it has recovered. Net contributions, deposits minus withdrawals over the same span, are given with the equity summary. These are changes in value, not investment return: a deposit or a purchase raises securities value without being a gain, so read the equity figures against the net contributions. To look at a window such as the past six months since the all-time high, call once to get the high's date, then again from that date. A series over 1000 points is rejected; use a larger window_days or a narrower range.

- from string, optional YYYY-MM-DD, inclusive; the earliest snapshot when absent
- to string, optional YYYY-MM-DD, inclusive; the latest snapshot when absent
- window_days int, optional, minimum 1, default 1; the number of days each point covers

1. Parse the parameters.
2. Read through store.GetValueHistory.
3. Return the range, window, points, the summaries and the net contributions, with a note that these are changes in value, not investment return.

#### Errors

- **1.** return the failure result.

---

- **2.** return the failure result; too many points is an ErrInvalidInput that names the fix.
