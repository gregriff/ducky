package utils

// clamp is a copy/pasted func from bubbles/textarea, in order to replicate its internal behavior.
func Clamp(v, low, high int) int {
	if high < low {
		low, high = high, low
	}
	return min(high, max(low, v))
}
