package slack

import (
	"encoding/base64"
	"os"
)

func decodeBase64(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}
