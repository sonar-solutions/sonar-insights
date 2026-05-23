package mathutil

import (
	"cmp"
	"math"
	"slices"
)

// CalculatePercentile returns the p-th percentile (0–1) of values using linear interpolation.
// Returns 0 if values is empty. The input slice is not modified.
func CalculatePercentile[T cmp.Ordered](values []T, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := slices.Sorted(slices.Values(values))
	position := p * float64(len(sorted)-1)
	lo := int(math.Floor(position))
	hi := int(math.Ceil(position))
	if lo == hi {
		return toFloat64(sorted[lo])
	}
	frac := position - float64(lo)
	return toFloat64(sorted[lo]) + (toFloat64(sorted[hi])-toFloat64(sorted[lo]))*frac
}

func toFloat64[T cmp.Ordered](v T) float64 {
	// T is constrained to cmp.Ordered which covers all numeric types and string.
	// We only use this with numeric types in practice.
	switch x := any(v).(type) {
	case int:
		return float64(x)
	case int8:
		return float64(x)
	case int16:
		return float64(x)
	case int32:
		return float64(x)
	case int64:
		return float64(x)
	case uint:
		return float64(x)
	case uint8:
		return float64(x)
	case uint16:
		return float64(x)
	case uint32:
		return float64(x)
	case uint64:
		return float64(x)
	case float32:
		return float64(x)
	case float64:
		return x
	default:
		return 0
	}
}
