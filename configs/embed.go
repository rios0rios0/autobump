// Package configs carries the configuration AutoBump ships with itself.
//
// It is data and nothing else. The declaration has to live beside the file because
// the go:embed directive cannot reach a parent directory, and the file has to keep its
// path because
// entities.DefaultConfigURL serves this same document from `main` -- so the built-in
// defaults and the published defaults can never describe different things.
package configs

import _ "embed"

// Default is configs/autobump.yaml as of the build. It is the first configuration
// layer, so a binary with no configuration of its own and no network still knows every
// language AutoBump supports.
//
//go:embed autobump.yaml
var Default []byte
