package entities

// Changelog test helpers were removed because the changelog logic has been
// moved to the gitforge shared library. The unexported functions
// (normalizeEntry, tokenize, extractMaxVersion, overlapRatio, recountChanges)
// are no longer local to this package.
//
// TODO: migrate changelog unit tests to gitforge or test only through public API.
