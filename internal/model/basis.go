package model

// CompareBasis selects which merge-base strategy scry uses for branch diffs.
type CompareBasis string

const (
	CompareBasisUpstream   CompareBasis = "upstream"
	CompareBasisLocalTrunk CompareBasis = "local-trunk"
)

func (b CompareBasis) Label() string {
	switch b {
	case CompareBasisLocalTrunk:
		return "local trunk"
	default:
		return "upstream"
	}
}

func (b CompareBasis) StatusLabel() string {
	switch b {
	case CompareBasisLocalTrunk:
		return "basis: local"
	default:
		return "basis: upstream"
	}
}
