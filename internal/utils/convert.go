// Package utils provides small shared helpers used across the lottery-stats-server
// service layer. These helpers centralize the repeated int↔int32 slice conversions
// that appear when bridging the lottery-tree algorithm (which works on []int) and
// the generated protobuf types (which use []int32).
package utils

// IntsToInt32s converts a []int slice to a []int32 slice of the same length.
func IntsToInt32s(in []int) []int32 {
	out := make([]int32, len(in))
	for i, v := range in {
		out[i] = int32(v)
	}
	return out
}

// Int32sToInts converts a []int32 slice to a []int slice of the same length.
func Int32sToInts(in []int32) []int {
	out := make([]int, len(in))
	for i, v := range in {
		out[i] = int(v)
	}
	return out
}

// IntsToInt32sFiltered converts a []int slice to a []int32 slice, dropping any
// values <= 0. Used when building proto NumberSet/Pair/FrequencyEntry lists
// where zero values are placeholders, not real numbers.
func IntsToInt32sFiltered(in []int) []int32 {
	out := make([]int32, 0, len(in))
	for _, v := range in {
		if v > 0 {
			out = append(out, int32(v))
		}
	}
	return out
}
