package support

import (
	"bufio"
	"errors"
	"fmt"
	"os"
)

var ErrFileNotFound = errors.New("file not found")

// ReadLines reads a whole file into memory.
func ReadLines(filePath string) ([]string, error) {
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

// WriteLines writes the lines to the given file.
func WriteLines(filePath string, lines []string) error {
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
