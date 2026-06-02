package sync

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/payload"
)

type ApplyResult struct {
	Written []string
}

func ApplyPlan(targetDir string, plan Plan) (ApplyResult, error) {
	var result ApplyResult
	for _, decision := range plan.Decisions {
		if !decision.IsWritable() {
			continue
		}
		file, ok := plan.Files[decision.Path]
		if !ok {
			return result, fmt.Errorf("missing source file for writable decision %s", decision.Path)
		}
		normalized, err := payload.NormalizeRelativePath(decision.Path)
		if err != nil {
			return result, err
		}
		target := filepath.Join(targetDir, filepath.FromSlash(normalized))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return result, fmt.Errorf("create parent for %s: %w", normalized, err)
		}
		mode := file.Mode.Perm()
		if mode == 0 {
			mode = 0o644
		}
		if err := os.WriteFile(target, file.Content, mode); err != nil {
			return result, fmt.Errorf("write %s: %w", normalized, err)
		}
		result.Written = append(result.Written, normalized)
	}
	return result, nil
}
