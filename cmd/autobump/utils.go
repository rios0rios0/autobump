package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
)

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

func findFile(locations []string, filename string) (string, error) {
	for _, location := range locations {
		if _, err := os.Stat(location); !os.IsNotExist(err) {
			return location, nil
		}
	}
	return "", errors.New(filename + " not found")
}
