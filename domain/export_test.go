package domain

import "github.com/Masterminds/semver/v3"

// Export unexported functions for testing.

// NormalizeEntryForTest exposes normalizeEntry for testing.
func NormalizeEntryForTest(entry string) string {
	return normalizeEntry(entry)
}

// TokenizeForTest exposes tokenize for testing.
func TokenizeForTest(normalized string) []string {
	return tokenize(normalized)
}

// ExtractMaxVersionForTest exposes extractMaxVersion for testing.
func ExtractMaxVersionForTest(entry string) *semver.Version {
	return extractMaxVersion(entry)
}

// OverlapRatioForTest exposes overlapRatio for testing.
func OverlapRatioForTest(a, b []string) float64 {
	return overlapRatio(a, b)
}

// RecountChangesForTest exposes recountChanges for testing.
func RecountChangesForTest(sections map[string]*[]string) (int, int, int) {
	return recountChanges(sections)
}
