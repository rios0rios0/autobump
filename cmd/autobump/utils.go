package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"

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

// getGpgKey returns GPG key entity from the given path
// it prompts for the passphrase to decrypt the key
func getGpgKey(gpgKeyPath string) (*openpgp.Entity, error) {
	privateKeyFile, err := os.Open(gpgKeyPath)
	if err != nil {
		log.Error("Failed to open private key file:", err)
	}
	entityList, err := openpgp.ReadArmoredKeyRing(privateKeyFile)
	if err != nil {
		log.Error("Failed to read private key file:", err)
	}

	fmt.Print("Enter the passphrase for your GPG key: ")
	passphrase, err := terminal.ReadPassword(0)
	if err != nil {
		return nil, err
	}
	fmt.Println()

	entity := entityList[0]
	err = entity.PrivateKey.Decrypt([]byte(passphrase))
	if err != nil {
		log.Error("Failed to decrypt GPG key:", err)
		return nil, err
	}

	log.Info("Successfully decrypted GPG key")
	return entity, nil
}
