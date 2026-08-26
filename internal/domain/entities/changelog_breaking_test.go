package entities_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rios0rios0/autobump/internal/domain/entities"
)

func TestNormalizeBreakingChangeMarker(t *testing.T) {
	t.Parallel()

	// Every case is the same three lines with different inputs, so they are a table rather
	// than a dozen copies of the same body.
	cases := []struct {
		name     string
		text     string
		breaking bool
		expected string
	}{
		{
			// The habit Conventional Commits teaches, met by the flag that says the same.
			name:     "should announce the change once when the flag and the body both say it",
			text:     "BREAKING CHANGE: dropped the v1 endpoint",
			breaking: true,
			expected: "**BREAKING CHANGE:** dropped the v1 endpoint",
		},
		{
			name:     "should collapse the markers when the body already carries a doubled one",
			text:     "**BREAKING CHANGE:** BREAKING CHANGE: dropped the v1 endpoint",
			breaking: true,
			expected: "**BREAKING CHANGE:** dropped the v1 endpoint",
		},
		{
			name:     "should add the marker when only the flag says the change is breaking",
			text:     "changed the configuration format",
			breaking: true,
			expected: "**BREAKING CHANGE:** changed the configuration format",
		},
		{
			// A fragment written by hand, or by a writer who forgot the flag.
			name:     "should add the marker when only the body says the change is breaking",
			text:     "BREAKING CHANGE: dropped the v1 endpoint",
			expected: "**BREAKING CHANGE:** dropped the v1 endpoint",
		},
		{
			name:     "should keep the canonical form stable when it is already canonical",
			text:     "**BREAKING CHANGE:** dropped the v1 endpoint",
			breaking: true,
			expected: "**BREAKING CHANGE:** dropped the v1 endpoint",
		},
		{
			name:     "should leave an ordinary entry untouched when nothing marks it",
			text:     "added the retry backoff",
			expected: "added the retry backoff",
		},
		{
			// The colon is what separates a marker from a sentence about breaking changes.
			name:     "should keep the sentence when an entry only begins with those words",
			text:     "changed how breaking changes are counted",
			expected: "changed how breaking changes are counted",
		},
		{
			// A body that is nothing but the marker must not render as a bare bullet.
			name:     "should keep the marker when the body is only the marker",
			text:     "BREAKING CHANGE:",
			breaking: true,
			expected: "**BREAKING CHANGE:**",
		},
		{
			name:     "should rewrite the marker when the body hyphenates it",
			text:     "BREAKING-CHANGE: dropped the v1 endpoint",
			expected: "**BREAKING CHANGE:** dropped the v1 endpoint",
		},
		{
			name:     "should rewrite the marker when the body lower-cases it",
			text:     "breaking change: dropped the v1 endpoint",
			expected: "**BREAKING CHANGE:** dropped the v1 endpoint",
		},
		{
			name:     "should rewrite the marker when the colon sits outside the emphasis",
			text:     "**BREAKING CHANGE**: dropped the v1 endpoint",
			expected: "**BREAKING CHANGE:** dropped the v1 endpoint",
		},
		{
			name:     "should rewrite the marker when the body emphasises it with underscores",
			text:     "__BREAKING CHANGE:__ dropped the v1 endpoint",
			expected: "**BREAKING CHANGE:** dropped the v1 endpoint",
		},
		{
			// The emphasis already announces it, so the colon is optional there.
			name:     "should rewrite the marker when the emphasis carries no colon",
			text:     "**BREAKING CHANGE** dropped the v1 endpoint",
			expected: "**BREAKING CHANGE:** dropped the v1 endpoint",
		},
		{
			name:     "should rewrite the marker when the body writes it in the plural",
			text:     "**BREAKING CHANGES:** dropped the v1 endpoint",
			expected: "**BREAKING CHANGE:** dropped the v1 endpoint",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			// given / when
			normalized := entities.NormalizeBreakingChangeMarker(testCase.text, testCase.breaking)

			// then
			assert.Equal(t, testCase.expected, normalized)
		})
	}
}
