---
id: 2026-11-23-idea-recovery-peaks
type: idea
summary: Follow-up idea for announcing smaller peaks during a long recovery below an old all-time high, for example in a multi-year bear market.
author: Thom
created: 2026-11-23
agents: claude
---

# Recovery peaks

## Status

Idea only, not part of the monitors batch. Everything below is to be decided.

## Idea

The all-time high monitors announce a high only when the value passes every earlier value. If the
last high was three years ago and the portfolio is slowly recovering, nothing is announced for years,
even though the value is making higher peaks along the way. The operator would like to hear about
those smaller peaks in a long bear market, leading up to the new all-time high.

## Ways to define a peak

- **Rolling-window high.** A new high of the past N days, such as a one year or two year high. It is
  one setting, needs no notion of a pullback, and gives peaks naturally: while the value is below an
  old high, it announces when it passes the highest value of the window. It also extends the all-time
  high monitors, which are the case of an unbounded window.
- **Recovery high.** A new high since the last trough that was at least X percent below the previous
  peak, announced after each such pullback.
- **Local peaks.** The highest value between two troughs of a given depth. This is the most precise
  and the hardest to define without hindsight.

The rolling-window high is the simplest and is the recommended starting point.

## What already exists

- The securities value series and its all-time high from the value history.
- The spacing between announced highs from the all-time high monitors, which also stops a rising
  market from announcing every day.
- The same series for instruments, so the idea applies to both.

## Gaps to resolve

- Which definition, and whether it is a setting of the all-time high monitors, such as a window, or a
  separate monitor.
- Whether the window counts calendar or series days, and what happens while the series is shorter
  than the window.
- How the messages tell a window high from an all-time high.

## Refs

- Related: 2026-11-23-monitor-portfolio-extremes, the all-time high monitors this extends.
- Related: 2026-11-23-monitor-instrument-extremes, the same for instrument prices.
