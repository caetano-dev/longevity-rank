package parser

import (
	"regexp"
	"strconv"
	"strings"
)

// rePriceFloat matches a decimal number in a price string (e.g. "29.99").
var rePriceFloat = regexp.MustCompile(`(\d+(?:\.\d+)?)`)

// extractFloat returns the first captured group of re in s as a float64.
// Returns (0, false) if there is no match or the value is <= 0.
func extractFloat(re *regexp.Regexp, s string) (float64, bool) {
	m := re.FindStringSubmatch(s)
	if len(m) < 2 {
		return 0, false
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil || v <= 0 {
		return 0, false
	}
	return v, true
}

// extractFloatFrom tries extractFloat against each source in order,
// returning the first successful match. This is the idiomatic way to
// implement "variant title → clean title → broad search" fallback chains.
func extractFloatFrom(re *regexp.Regexp, sources ...string) (float64, bool) {
	for _, s := range sources {
		if v, ok := extractFloat(re, s); ok {
			return v, ok
		}
	}
	return 0, false
}

// containsAny reports whether s contains any of the given substrings.
func containsAny(s string, substrs []string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}