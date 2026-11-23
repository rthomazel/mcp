package domain

// AllocationGroup is a named set of instruments, by ticker, judged against a band.
type AllocationGroup struct {
	Name       string
	Tickers    []string
	MinPercent *Decimal // nil means no minimum
	MaxPercent *Decimal // nil means no maximum
}

// Side of a band a group is outside of.
const (
	SideAbove = "above"
	SideBelow = "below"
)

// MemberAllocation is an instrument's share of the securities value.
type MemberAllocation struct {
	Symbol            string  `json:"symbol"`
	AllocationPercent Decimal `json:"allocation_percent"`
}

// GroupAllocation is a group's allocation on the latest date and its stay outside the band.
type GroupAllocation struct {
	Name              string
	AllocationPercent Decimal
	Side              string // SideAbove, SideBelow, or empty when inside the band
	StayStart         *Date  // first date of the current stay on Side; nil when inside the band
	Members           []MemberAllocation
}

// AllocationDrift is every group's allocation on the latest date of the securities series.
type AllocationDrift struct {
	Date         Date
	Groups       []GroupAllocation // in the order of the input groups
	Unregistered []string          // tickers with no registered instrument; their allocation is zero
}
