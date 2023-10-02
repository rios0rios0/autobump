package main

import (
	"bufio"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh/terminal"
)

// readLines reads a whole file into memory
func readLines(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

// writeLines writes the lines to the given file
func writeLines(filePath string, lines []string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for _, line := range lines {
		fmt.Fprintln(writer, line)
	}
	return writer.Flush()
}

// findFile returns the first location where the file exists
func findFile(locations []string, filename string) (string, error) {
	for _, location := range locations {
		if _, err := os.Stat(location); !os.IsNotExist(err) {
			return location, nil
		}
	}
	return "", errors.New(filename + " not found")
}

// downloadFile downloads a file from the given URL
func downloadFile(url string) ([]byte, error) {
	var data []byte

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// getGpgKey returns GPG key entity from the given path
// it prompts for the passphrase to decrypt the key
func getGpgKey(gpgKeyId, gpgKeyPath string) (*openpgp.Entity, error) {
	var err error

	location := gpgKeyPath
	if location == "" {
		location = os.ExpandEnv(fmt.Sprintf("$HOME/.gnupg/autobump-%s.asc", gpgKeyId))
		log.Warnf("No key path provided, attempting to read (%s) at: %s", gpgKeyId, location)

		if _, err = os.Stat(location); os.IsNotExist(err) {
			// TODO: until today Go is not capable to read the key from the keyring (kbx)
			cmd := exec.Command("gpg", "--export-secret-key", "--output", location, "--armor", gpgKeyId)
			err = cmd.Run()
			if err != nil {
				return nil, fmt.Errorf("failed to execute command GPG: %w", err)
			}
		}
	}

	privateKeyFile, err := os.Open(location)
	if err != nil {
		return nil, errors.New("failed to open private key file")
	}
	defer privateKeyFile.Close()

	entityList, err := openpgp.ReadArmoredKeyRing(privateKeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file: %s", err)
	}

	entity := entityList[0]
	if entity == nil {
		return nil, fmt.Errorf("failed to find key with fingerprint %s", gpgKeyId)
	}

	fmt.Print("Enter the passphrase for your GPG key: ")
	var passphrase []byte
	passphrase, err = terminal.ReadPassword(0)
	// assume the passphrase to be empty if unable to read from the terminal
	if err != nil {
		if strings.TrimSpace(err.Error()) == "inappropriate ioctl for device" {
			passphrase = []byte("")
		} else {
			return nil, err
		}
	}
	fmt.Println()

	if entity.PrivateKey == nil {
		return nil, fmt.Errorf("failed to find private key for %s", gpgKeyId)
	}

	err = entity.PrivateKey.Decrypt(passphrase)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt GPG key: %v", err)
	}

	log.Info("Successfully decrypted GPG key")
	return entity, nil
}
