package repomap

import (
	"testing"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/repomap/model"
)

func TestLinkTestsByNamingConvention(t *testing.T) {
	files := []model.FileRecord{
		{Path: "internal/core/core.go", Language: "go"},
		{Path: "internal/core/core_test.go", Language: "go"},
		{Path: "pkg/app.py", Language: "python"},
		{Path: "pkg/test_app.py", Language: "python"},
		{Path: "web/button.ts", Language: "typescript"},
		{Path: "web/button.spec.ts", Language: "typescript"},
		{Path: "lib/missing_test.go", Language: "go"},
	}

	got := LinkTests(files)
	want := map[string]string{
		"internal/core/core_test.go": "internal/core/core.go",
		"pkg/test_app.py":            "pkg/app.py",
		"web/button.spec.ts":         "web/button.ts",
	}
	if len(got) != len(want) {
		t.Fatalf("LinkTests() len = %d, want %d: %#v", len(got), len(want), got)
	}
	for _, record := range got {
		if want[record.Path] != record.TargetPath {
			t.Fatalf("LinkTests() target for %s = %s, want %s", record.Path, record.TargetPath, want[record.Path])
		}
	}
}
