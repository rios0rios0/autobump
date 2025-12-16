package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	log "github.com/sirupsen/logrus"
	"golang.org/x/term"
)

var (
	ErrFileNotFound                         = errors.New("file not found")
	ErrCannotFindPrivKey                    = errors.New("cannot find private key")
	ErrCannotFindPrivKeyMatchingFingerprint = errors.New(
		"cannot find private key matching fingerprint",
	)
)

const downloadTimeout = 30

// readLines reads a whole file into memory.
func readLines(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	err = scanner.Err()
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	return lines, nil
}

// writeLines writes the lines to the given file.
func writeLines(filePath string, lines []string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for _, line := range lines {
		fmt.Fprintln(writer, line)
	}

	err = writer.Flush()
	if err != nil {
		return fmt.Errorf("failed to write to file: %w", err)
	}
	return nil
}

// downloadFile downloads a file from the given URL.
func downloadFile(url string) ([]byte, error) {
	var data []byte

	ctx, cancel := context.WithTimeout(context.Background(), downloadTimeout*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create download request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	data, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return data, nil
}

func exportGpgKey(ctx context.Context, gpgKeyID string, gpgKeyExportPath string) error {
	// TODO: until today Go is not capable to read the key from the keyring (kbx)
	cmd := exec.CommandContext(
		ctx,
		"gpg",
		"--export-secret-key",
		"--output",
		gpgKeyExportPath,
		"--armor",
		gpgKeyID,
	)
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to execute command GPG: %w", err)
	}
	return nil
}

func getGpgKeyReader(ctx context.Context, gpgKeyID string, gpgKeyPath string) (*io.Reader, error) {
	// if no key path is provided, try to read the key from the default location
	if gpgKeyPath == "" {
		gpgKeyPath = os.ExpandEnv(fmt.Sprintf("$HOME/.gnupg/autobump-%s.asc", gpgKeyID))
		log.Warnf("No key path provided, attempting to read (%s) at: %s", gpgKeyID, gpgKeyPath)

		// if the key does not exist, try to export it from the keyring
		if _, err := os.Stat(gpgKeyPath); os.IsNotExist(err) {
			err = exportGpgKey(ctx, gpgKeyID, gpgKeyPath)
			if err != nil {
				return nil, err
			}
		}
	}

	gpgKeyFile, err := os.Open(gpgKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open private key file: %w", err)
	}
	defer gpgKeyFile.Close()

	reader := io.Reader(gpgKeyFile)
	return &reader, nil
}

// getGpgKey returns GPG key entity from the given path
// it prompts for the passphrase to decrypt the key.
func getGpgKey(gpgKeyReader io.Reader) (*openpgp.Entity, error) {
	var err error

	entityList, err := openpgp.ReadArmoredKeyRing(gpgKeyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file: %w", err)
	}

	entity := entityList[0]
	if entity == nil {
		return nil, ErrCannotFindPrivKeyMatchingFingerprint
	}

	fmt.Print("Enter the passphrase for your GPG key: ") //nolint:forbidigo // this line is not for debugging
	var passphrase []byte
	passphrase, err = term.ReadPassword(0)
	// assume the passphrase to be empty if unable to read from the terminal
	if err != nil {
		if strings.TrimSpace(err.Error()) == "inappropriate ioctl for device" {
			passphrase = []byte("")
		} else {
			return nil, fmt.Errorf("failed to read passphrase: %w", err)
		}
	}
	fmt.Println() //nolint:forbidigo // this line is not for debugging

	if entity.PrivateKey == nil {
		return nil, ErrCannotFindPrivKey
	}

	err = entity.PrivateKey.Decrypt(passphrase)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt GPG key: %w", err)
	}

	log.Info("Successfully decrypted GPG key")
	return entity, nil
}
