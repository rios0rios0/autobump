package entities_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rios0rios0/autobump/internal/domain/entities"
)

func TestNormalizeBreakingChangeMarker(t *testing.T) {
	t.Parallel()

	t.Run("should announce the change once when the flag and the body both say it", func(t *testing.T) {
		t.Parallel()

		// given
		text := "BREAKING CHANGE: dropped the v1 endpoint"

		// when
		normalized := entities.NormalizeBreakingChangeMarker(text, true)

		// then
		assert.Equal(t, "**BREAKING CHANGE:** dropped the v1 endpoint", normalized)
	})

	t.Run("should collapse the markers when the body already carries a doubled one", func(t *testing.T) {
		t.Parallel()

		// given
		text := "**BREAKING CHANGE:** BREAKING CHANGE: dropped the v1 endpoint"

		// when
		normalized := entities.NormalizeBreakingChangeMarker(text, true)

		// then
		assert.Equal(t, "**BREAKING CHANGE:** dropped the v1 endpoint", normalized)
	})

	t.Run("should add the marker when only the flag says the change is breaking", func(t *testing.T) {
		t.Parallel()

		// given
		text := "changed the configuration format"

		// when
		normalized := entities.NormalizeBreakingChangeMarker(text, true)

		// then
		assert.Equal(t, "**BREAKING CHANGE:** changed the configuration format", normalized)
	})

	t.Run("should leave an ordinary entry untouched when nothing marks it", func(t *testing.T) {
		t.Parallel()

		// given
		text := "added the retry backoff"

		// when
		normalized := entities.NormalizeBreakingChangeMarker(text, false)

		// then
		assert.Equal(t, text, normalized)
	})

	t.Run("should keep the canonical form stable when it is already canonical", func(t *testing.T) {
		t.Parallel()

		// given
		text := "**BREAKING CHANGE:** dropped the v1 endpoint"

		// when
		normalized := entities.NormalizeBreakingChangeMarker(text, true)

		// then
		assert.Equal(t, text, normalized)
	})

	t.Run("should rewrite the marker when the body spells it another way", func(t *testing.T) {
		t.Parallel()

		spellings := map[string]string{
			"plain":                "BREAKING CHANGE: dropped the v1 endpoint",
			"hyphenated":           "BREAKING-CHANGE: dropped the v1 endpoint",
			"lower cased":          "breaking change: dropped the v1 endpoint",
			"colon outside":        "**BREAKING CHANGE**: dropped the v1 endpoint",
			"underscore emphasis":  "__BREAKING CHANGE:__ dropped the v1 endpoint",
			"emphasis without any": "**BREAKING CHANGE** dropped the v1 endpoint",
			"plural":               "**BREAKING CHANGES:** dropped the v1 endpoint",
		}

		for name, text := range spellings {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				// given / when
				normalized := entities.NormalizeBreakingChangeMarker(text, false)

				// then
				assert.Equal(t, "**BREAKING CHANGE:** dropped the v1 endpoint", normalized)
			})
		}
	})

	t.Run("should keep the sentence when an entry only begins with those words", func(t *testing.T) {
		t.Parallel()

		// given the colon is what separates a marker from a sentence about breaking changes
		text := "changed how breaking changes are counted"

		// when
		normalized := entities.NormalizeBreakingChangeMarker(text, false)

		// then
		assert.Equal(t, text, normalized)
	})

	t.Run("should not render a bare bullet when the body is only the marker", func(t *testing.T) {
		t.Parallel()

		// given
		text := "BREAKING CHANGE:"

		// when
		normalized := entities.NormalizeBreakingChangeMarker(text, true)

		// then
		assert.Equal(t, "**BREAKING CHANGE:**", normalized)
	})
}
