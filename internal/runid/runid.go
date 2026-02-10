package runid

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
)

func FromFiles(paths ...string) (string, error) {
	h := sha256.New()

	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			return "", err
		}
		_, copyErr := io.Copy(h, f)
		closeErr := f.Close()
		if copyErr != nil {
			return "", copyErr
		}
		if closeErr != nil {
			return "", closeErr
		}
	}

	sum := h.Sum(nil)
	// short, readable id (still deterministic)
	return hex.EncodeToString(sum)[:16], nil

}
