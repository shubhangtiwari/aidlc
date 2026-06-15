package model

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
)

func ContentHash(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func ContentHashReader(r io.Reader) (string, error) {
	hash := sha256.New()
	if _, err := io.Copy(hash, r); err != nil {
		return "", fmt.Errorf("hash content: %w", err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
