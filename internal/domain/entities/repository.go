package entities

// Language is the interface for language-specific project operations.
// This is autobump-specific and not part of gitforge.
type Language interface {
	GetProjectName() (string, error)
}
