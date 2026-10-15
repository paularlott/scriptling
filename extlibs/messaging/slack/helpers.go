package slack

import (
	"encoding/base64"

	"github.com/paularlott/scriptling/object"
)

func decodeBase64(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

func readFile(path string) ([]byte, error) {
	return object.ReadFileLimitedErr(path)
}
