package entitybuilders

import testkit "github.com/rios0rios0/testkit/pkg/test"

// cloneBase clones the embedded BaseBuilder and asserts it back to its concrete type.
//
// testkit.Builder.Clone returns the interface, so every builder has to assert. Doing it
// here once keeps the assertion checked — errcheck flags the single-value form — without
// repeating the same four lines in each builder. A failed assertion means testkit changed
// what Clone returns, which no test can carry on from, so it panics rather than handing
// back a nil builder that fails much later and somewhere else.
func cloneBase(base *testkit.BaseBuilder) *testkit.BaseBuilder {
	cloned, ok := base.Clone().(*testkit.BaseBuilder)
	if !ok {
		panic("testkit.BaseBuilder.Clone did not return a *testkit.BaseBuilder")
	}

	return cloned
}
