package proxy

import (
	"encoding/base64"
	"io"
	"io/ioutil"
	"log"
	"net/http"
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

func decryptBase64(input []byte) (ouput []byte, err error) {
	d, err := base64.StdEncoding.DecodeString(string(input))

	if err != nil {
		return nil, err
	}

	return d, err
}

func ProxyFromUrl(link string) ([]string, error) {
	c := http.Client{}

	resp, err := c.Get(link)

	if err != nil {
		log.Print("error in sending request: ", err)
		return nil, err
	}

	content, err := io.ReadAll(resp.Body)

	if err != nil {
		log.Print("error in reading file", err)
		return nil, err
	}

	content, err = decryptBase64(content)

	if err != nil {
		log.Print("error decrypting base64", err)
		return nil, err
	}

	lines := strings.Split(string(content), "\n")

	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}

	return lines, nil
}
