package install

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

type ChecksumSet map[string]string

func ParseChecksums(r io.Reader) (ChecksumSet, error) {
	checksums := ChecksumSet{}
	scanner := bufio.NewScanner(r)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		sum, name, ok := parseChecksumLine(line)
		if !ok {
			return nil, fmt.Errorf("parse checksums line %d: expected '<sha256> <filename>'", lineNumber)
		}
		checksums[name] = strings.ToLower(sum)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read checksums: %w", err)
	}
	return checksums, nil
}

func VerifyChecksum(r io.Reader, filename string, checksums ChecksumSet) error {
	want, ok := checksums[filename]
	if !ok {
		return fmt.Errorf("checksum for %s not found", filename)
	}
	if len(want) != sha256.Size*2 {
		return fmt.Errorf("checksum for %s is not a SHA-256 digest", filename)
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, r); err != nil {
		return fmt.Errorf("hash %s: %w", filename, err)
	}
	got := hex.EncodeToString(hash.Sum(nil))
	if got != strings.ToLower(want) {
		return fmt.Errorf("checksum mismatch for %s", filename)
	}
	return nil
}

func parseChecksumLine(line string) (string, string, bool) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return "", "", false
	}
	sum := fields[0]
	if len(sum) != sha256.Size*2 {
		return "", "", false
	}
	if _, err := hex.DecodeString(sum); err != nil {
		return "", "", false
	}
	name := strings.TrimPrefix(fields[1], "*")
	if name == "" || strings.Contains(name, "/") || strings.Contains(name, `\`) {
		return "", "", false
	}
	return sum, name, true
}
