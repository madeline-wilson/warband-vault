package update

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
)

func hashFile(path string) ([sha256.Size]byte, error) {
	var zero [sha256.Size]byte
	file, err := os.Open(path)
	if err != nil {
		return zero, fmt.Errorf("open file for sha256: %w", err)
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return zero, fmt.Errorf("hash file: %w", err)
	}
	var sum [sha256.Size]byte
	copy(sum[:], hasher.Sum(nil))
	return sum, nil
}
