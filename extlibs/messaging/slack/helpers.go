package slack

import (
	"encoding/base64"

	"github.com/paularlott/scriptling/extlibs/fssecurity"
	"github.com/paularlott/scriptling/object"
)

func decodeBase64(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

// readFile reads a local file a script uploads, once files allows it (nil =
// unrestricted).
func readFile(files *fssecurity.Config, path string) ([]byte, error) {
	if err := files.CheckPath(path); err != nil {
		return nil, err
	}
	return object.ReadFileLimitedErr(path)
}
