package shared

import (
	b64 "encoding/base64"
)

// EncodeBase64 encodes bytes to a base64 string.
func EncodeBase64(data []byte) string {
	return b64.StdEncoding.EncodeToString(data)
}
