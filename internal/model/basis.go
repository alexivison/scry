package model

// CompareBasis selects which merge-base strategy scry uses for branch diffs.
type CompareBasis string

const (
	CompareBasisUpstream   CompareBasis = "upstream"
	CompareBasisLocalTrunk CompareBasis = "local-trunk"
	CompareBasisHeadDirty  CompareBasis = "head-dirty"
)

func (b CompareBasis) Label() string {
	switch b {
	case CompareBasisLocalTrunk:
		return "local trunk"
	case CompareBasisHeadDirty:
		return "HEAD/dirty"
	default:
		return "upstream"
	}
}

func (b CompareBasis) StatusLabel() string {
	switch b {
	case CompareBasisLocalTrunk:
		return "basis: local"
	case CompareBasisHeadDirty:
		return "basis: HEAD/dirty"
	default:
		return "basis: upstream"
	}
}

func (b CompareBasis) Next() CompareBasis {
	switch b {
	case CompareBasisUpstream:
		return CompareBasisLocalTrunk
	case CompareBasisLocalTrunk:
		return CompareBasisHeadDirty
	default:
		return CompareBasisUpstream
	}
}
