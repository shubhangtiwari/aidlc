package sync

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/contract"
	"github.com/shubhangtiwari/aidlc/aidlc/internal/payload"
	"github.com/shubhangtiwari/aidlc/aidlc/internal/source"
)

type PlanRequest struct {
	Mode             Mode
	TargetDir        string
	Source           source.Snapshot
	PreviousManifest *contract.TargetManifest
}

type Plan struct {
	Mode      Mode
	Upstream  contract.UpstreamRef
	Decisions []Decision
	Files     map[string]source.File
}

func BuildPlan(req PlanRequest) (Plan, error) {
	if req.Mode == "" {
		req.Mode = ModeInit
	}
	if req.Mode != ModeInit && req.Mode != ModeUpdate {
		return Plan{}, fmt.Errorf("unsupported sync mode %q", req.Mode)
	}
	if req.TargetDir == "" {
		return Plan{}, fmt.Errorf("target directory is required")
	}
	if err := source.ValidateSnapshot(req.Source); err != nil {
		return Plan{}, err
	}

	upstreamFiles := make(map[string]source.File, len(req.Source.Files))
	for _, file := range req.Source.Files {
		normalized, err := payload.NormalizeRelativePath(file.Path)
		if err != nil {
			return Plan{}, err
		}
		file.Path = normalized
		upstreamFiles[normalized] = file
	}

	previous := make(map[string]contract.ManifestFile)
	if req.PreviousManifest != nil {
		for _, file := range req.PreviousManifest.Files {
			normalized, err := payload.NormalizeRelativePath(file.Path)
			if err != nil {
				return Plan{}, fmt.Errorf("invalid previous manifest path %q: %w", file.Path, err)
			}
			file.Path = normalized
			previous[normalized] = file
		}
	}

	paths := make([]string, 0, len(upstreamFiles)+len(previous))
	seen := make(map[string]struct{}, len(upstreamFiles)+len(previous))
	for name := range upstreamFiles {
		paths = append(paths, name)
		seen[name] = struct{}{}
	}
	if req.Mode == ModeUpdate {
		for name := range previous {
			if _, ok := seen[name]; !ok {
				paths = append(paths, name)
			}
		}
	}
	sort.Strings(paths)

	plan := Plan{
		Mode:      req.Mode,
		Upstream:  req.Source.Upstream,
		Decisions: make([]Decision, 0, len(paths)),
		Files:     upstreamFiles,
	}
	for _, name := range paths {
		upstream, hasUpstream := upstreamFiles[name]
		prev, hasPrevious := previous[name]
		localChecksum, localExists, err := targetChecksum(req.TargetDir, name)
		if err != nil {
			return Plan{}, err
		}

		if !hasUpstream {
			plan.Decisions = append(plan.Decisions, Decision{
				Path:             name,
				State:            StateRemovedUpstream,
				Reason:           "tracked file is no longer present upstream; local file is not deleted",
				PreviousChecksum: prev.Checksum,
				LocalChecksum:    localChecksum,
			})
			continue
		}

		upstreamChecksum := BytesChecksum(upstream.Content)
		decision := Decision{
			Path:             name,
			PreviousChecksum: prev.Checksum,
			LocalChecksum:    localChecksum,
			UpstreamChecksum: upstreamChecksum,
			Mode:             formatMode(upstream.Mode),
		}

		if !localExists {
			decision.State = StateCreate
			decision.Reason = "file does not exist locally"
			plan.Decisions = append(plan.Decisions, decision)
			continue
		}
		if localChecksum == upstreamChecksum {
			decision.State = StateSkip
			decision.Reason = "local file already matches upstream"
			plan.Decisions = append(plan.Decisions, decision)
			continue
		}
		if req.Mode == ModeInit {
			decision.State = StateConflict
			decision.Reason = "init never overwrites divergent local files"
			plan.Decisions = append(plan.Decisions, decision)
			continue
		}
		if hasPrevious && localChecksum == prev.Checksum {
			decision.State = StateUpdateClean
			decision.Reason = "local file matches previous manifest and upstream changed"
			plan.Decisions = append(plan.Decisions, decision)
			continue
		}

		decision.State = StateConflict
		if hasPrevious {
			decision.Reason = "local file diverged from previous manifest"
		} else {
			decision.Reason = "local file is not tracked by the previous manifest"
		}
		plan.Decisions = append(plan.Decisions, decision)
	}

	return plan, nil
}

func targetChecksum(root, name string) (string, bool, error) {
	target := filepath.Join(root, filepath.FromSlash(name))
	checksum, err := FileChecksum(target)
	if err == nil {
		return checksum, true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	return "", false, fmt.Errorf("checksum target file %s: %w", name, err)
}

func formatMode(mode os.FileMode) string {
	if mode == 0 {
		return ""
	}
	return fmt.Sprintf("%#o", mode.Perm())
}
