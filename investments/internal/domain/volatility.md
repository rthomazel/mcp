# Types

## PriceVolatility

1. Date Date, the latest price date in the store
2. StartDate Date, the last price date on or before the window start
3. MovePercent Decimal, the adjusted close change from StartDate to Date as a percent
4. Moves int, the moves in the look-back the volatility is taken from
5. VolatilityPercent *Decimal, the sample standard deviation of those moves, nil when it is zero or there are fewer than two moves
6. Variation *Decimal, MovePercent divided by VolatilityPercent, signed, nil without volatility
7. AtLeastUp bool, Variation is at or above the threshold
8. AtLeastDown bool, Variation is at or below minus the threshold

The judged move is not part of the baseline. A move of a date is the change from the last price date on or before the date counted back by the window, over adjusted closes, so splits and dividends are not moves. The threshold comparisons are done by the store without rounding and are false without volatility.
