package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

func WriteFile(t testing.TB, root, name, content string) string {
	t.Helper()

	target := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("create parent directory for %s: %v", name, err)
	}
	if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return target
}

func ReadFile(t testing.TB, root, name string) string {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(content)
}
