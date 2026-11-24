# Types

## PriceTarget

1. Date Date, the latest price date in the store
2. Low Decimal, the day's lowest unadjusted price
3. High Decimal, the day's highest unadjusted price
4. Close Decimal, the day's unadjusted close
5. Reached bool, the target price is at or above Low and at or below High

An instrument's price range on the latest price date and whether a target lies within it. Unadjusted, since a target is a price the operator sees on a quote. Reached is decided by the store without rounding.
