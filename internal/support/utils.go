package support

import (
	"context"
	"errors"
	"io"

	"github.com/ProtonMail/go-crypto/openpgp"

	forgeSigning "github.com/rios0rios0/gitforge/infrastructure/signing"
	forgeSupport "github.com/rios0rios0/gitforge/support"
)

var (
	ErrCannotFindPrivKey                    = forgeSigning.ErrCannotFindPrivKey
	ErrCannotFindPrivKeyMatchingFingerprint = forgeSigning.ErrCannotFindPrivKeyMatchingFingerprint
	ErrFileNotFound                         = errors.New("file not found")
)

// ReadLines reads a whole file into memory.
func ReadLines(filePath string) ([]string, error) {
	return forgeSupport.ReadLines(filePath)
}

// WriteLines writes the lines to the given file.
func WriteLines(filePath string, lines []string) error {
	return forgeSupport.WriteLines(filePath, lines)
}

// DownloadFile downloads a file from the given URL.
func DownloadFile(url string) ([]byte, error) {
	return forgeSupport.DownloadFile(url)
}

// StripUsernameFromURL removes the username from a URL if present.
func StripUsernameFromURL(rawURL string) string {
	return forgeSupport.StripUsernameFromURL(rawURL)
}

// ExportGpgKey exports a GPG key from the keyring to a file.
func ExportGpgKey(ctx context.Context, gpgKeyID string, gpgKeyExportPath string) error {
	return forgeSigning.ExportGpgKey(ctx, gpgKeyID, gpgKeyExportPath)
}

// GetGpgKeyReader returns a reader for the GPG key.
func GetGpgKeyReader(ctx context.Context, gpgKeyID string, gpgKeyPath string) (io.Reader, error) {
	return forgeSigning.GetGpgKeyReader(ctx, gpgKeyID, gpgKeyPath, "autobump")
}

// GetGpgKey returns GPG key entity from the given reader.
func GetGpgKey(gpgKeyReader io.Reader) (*openpgp.Entity, error) {
	return forgeSigning.GetGpgKey(gpgKeyReader)
}
