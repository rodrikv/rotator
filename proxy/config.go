package proxy

import (
	"io/ioutil"
	"strings"
)

func ProxyFromFile(filePath string) ([]string, error) {
	lines, err := readLinesFromFile(filePath)
	if err != nil {
		return nil, err
	}

	return lines, nil
}

func readLinesFromFile(filePath string) ([]string, error) {
	// Read the entire file content
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	// Split the content into lines
	lines := strings.Split(string(content), "\n")

	// Remove any leading or trailing whitespaces from each line
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}

	return lines, nil
}
