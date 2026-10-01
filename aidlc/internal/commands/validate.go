package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/contract"
)

type ValidateOptions struct {
	Dir          string
	Spec         string
	TaskRecord   string
	Base         string
	ChangedFiles []string
	HighRisk     bool
	Format       string
}

type ValidateDependencies struct {
	RunGit func(ctx context.Context, dir string, args ...string) ([]byte, error)
}

type ValidateResult struct {
	Passed        bool     `json:"passed"`
	ScopeRoot     string   `json:"scope_root"`
	Spec          string   `json:"spec,omitempty"`
	TaskRecord    string   `json:"task_record,omitempty"`
	Revision      string   `json:"revision,omitempty"`
	ContentDigest string   `json:"content_digest,omitempty"`
	ChangedFiles  []string `json:"changed_files,omitempty"`
	Failures      []string `json:"failures,omitempty"`
	Notes         []string `json:"notes,omitempty"`
}

func RunValidateCLI(ctx context.Context, args []string, stdout, stderr io.Writer, deps ValidateDependencies) int {
	if isHelpArg(args) {
		printValidateUsage(stdout)
		return contract.ExitOK
	}

	opts := ValidateOptions{Dir: ".", Format: "text"}
	fs := flag.NewFlagSet("aidlc validate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&opts.Dir, "dir", opts.Dir, "repository directory to validate")
	fs.StringVar(&opts.Spec, "spec", "", "approved spec path to validate")
	fs.StringVar(&opts.TaskRecord, "task-record", "", "task record JSON evidence path")
	fs.StringVar(&opts.Base, "base", "", "base revision for changed-file discovery")
	fs.BoolVar(&opts.HighRisk, "high-risk", false, "require high-risk task evidence")
	fs.StringVar(&opts.Format, "format", opts.Format, "output format: text or json")
	fs.Var((*repeatString)(&opts.ChangedFiles), "changed-file", "slash-relative changed file path; may repeat")
	fs.Usage = func() { printValidateUsage(stderr) }
	if err := fs.Parse(args); err != nil {
		return contract.ExitUsage
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return contract.ExitUsage
	}

	result, err := RunValidate(ctx, opts, deps)
	if err != nil {
		fmt.Fprintf(stderr, "aidlc validate: %v\n", err)
		return contract.ExitUsage
	}
	if err := printValidateResult(stdout, result, opts.Format); err != nil {
		fmt.Fprintf(stderr, "aidlc validate: %v\n", err)
		return contract.ExitUsage
	}
	if !result.Passed {
		return contract.ExitConflict
	}
	return contract.ExitOK
}

func RunValidate(ctx context.Context, opts ValidateOptions, deps ValidateDependencies) (ValidateResult, error) {
	deps = validateDependenciesWithDefaults(deps)
	root, err := resolveScopeRoot(opts.Dir)
	if err != nil {
		return ValidateResult{}, err
	}
	format := strings.TrimSpace(strings.ToLower(opts.Format))
	if format == "" {
		format = "text"
	}
	if format != "text" && format != "json" {
		return ValidateResult{}, fmt.Errorf("--format must be text or json")
	}

	result := ValidateResult{ScopeRoot: root}
	currentRevision, err := resolveGitRevision(ctx, root, "HEAD", deps)
	if err == nil {
		result.Revision = currentRevision
	} else if opts.Base != "" || opts.HighRisk {
		return ValidateResult{}, fmt.Errorf("resolve HEAD: %w", err)
	}

	var spec *specEvidence
	if strings.TrimSpace(opts.Spec) != "" {
		specRel, err := cleanRepoPath("--spec", opts.Spec)
		if err != nil {
			return ValidateResult{}, err
		}
		result.Spec = specRel
		spec, err = readSpecEvidence(root, specRel)
		if err != nil {
			return ValidateResult{}, err
		}
		if spec.Status != "approved" {
			result.Failures = append(result.Failures, fmt.Sprintf("spec %s status is %q, want approved", specRel, spec.Status))
		}
	}

	var record *contract.TaskRecordV1
	if strings.TrimSpace(opts.TaskRecord) != "" {
		recordRel, err := cleanRepoPath("--task-record", opts.TaskRecord)
		if err != nil {
			return ValidateResult{}, err
		}
		result.TaskRecord = recordRel
		record, err = readTaskRecord(root, recordRel)
		if err != nil {
			return ValidateResult{}, err
		}
		if err := record.Validate(); err != nil {
			result.Failures = append(result.Failures, err.Error())
		}
		if record.Spec != "" && result.Spec != "" && path.Clean(record.Spec) != result.Spec {
			result.Failures = append(result.Failures, "task record spec does not match --spec")
		}
	}

	highRisk := opts.HighRisk || (record != nil && record.Risk == contract.TaskRiskHigh)
	if highRisk {
		if spec == nil {
			result.Failures = append(result.Failures, "high-risk validation requires --spec")
		}
		if record == nil {
			result.Failures = append(result.Failures, "high-risk validation requires --task-record")
		}
	}
	if record != nil {
		if record.ScopeRoot != "." && path.Clean(record.ScopeRoot) != "." {
			result.Failures = append(result.Failures, "task record scope_root must be . for the selected --dir")
		}
		if highRisk && opts.Base == "" {
			if record.BaseRevision == "" {
				result.Failures = append(result.Failures, "high-risk validation requires --base or task record base_revision")
			} else {
				opts.Base = record.BaseRevision
			}
		}
		if opts.Base != "" {
			baseRevision, err := resolveGitRevision(ctx, root, opts.Base, deps)
			if err != nil {
				return ValidateResult{}, fmt.Errorf("resolve --base: %w", err)
			}
			if record.BaseRevision == "" {
				result.Failures = append(result.Failures, "task record base_revision is required when --base is supplied")
			} else {
				recordBase, err := resolveGitRevision(ctx, root, record.BaseRevision, deps)
				if err != nil {
					result.Failures = append(result.Failures, fmt.Sprintf("task record base_revision is unreadable: %v", err))
				} else if recordBase != baseRevision {
					result.Failures = append(result.Failures, "task record base_revision does not resolve to --base")
				}
			}
		}
	}

	changed, err := changedFiles(ctx, root, opts, deps)
	if err != nil {
		return ValidateResult{}, err
	}
	result.ChangedFiles = changed
	for _, changedPath := range changed {
		if err := validateOwnedScope(root, changedPath); err != nil {
			result.Failures = append(result.Failures, err.Error())
		}
		if spec != nil && changedPath != result.Spec && !spec.Allows(changedPath) {
			result.Failures = append(result.Failures, fmt.Sprintf("changed file %s is not listed in the approved spec", changedPath))
		}
	}

	if record != nil {
		owned := pathSet(record.OwnedFiles)
		for _, changedPath := range changed {
			if result.Spec != "" && changedPath == result.Spec {
				continue
			}
			if _, ok := owned[changedPath]; !ok {
				result.Failures = append(result.Failures, fmt.Sprintf("changed file %s is not covered by task record owned_files", changedPath))
			}
		}
		digest, err := digestOwnedFiles(root, record.OwnedFiles)
		if err != nil {
			return ValidateResult{}, err
		}
		result.ContentDigest = digest
		if record.ContentDigest != "" && record.ContentDigest != digest {
			result.Failures = append(result.Failures, "task record content_digest is stale")
		}
		validateCheckEvidence(ctx, deps, &result, record.Checks, currentRevision, digest)
		if highRisk {
			validateReviewEvidence(ctx, deps, &result, record.Review, currentRevision, digest)
		}
	}
	if !highRisk && record == nil {
		result.Notes = append(result.Notes, "task records are optional for low-risk validation")
	}
	result.Passed = len(result.Failures) == 0
	return result, nil
}

type repeatString []string

func (r *repeatString) String() string { return strings.Join(*r, ",") }
func (r *repeatString) Set(value string) error {
	*r = append(*r, value)
	return nil
}

type specEvidence struct {
	Status        string
	AffectedFiles map[string]struct{}
}

func readSpecEvidence(root, rel string) (*specEvidence, error) {
	if err := ensurePathReadableInside(root, rel); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return nil, fmt.Errorf("read spec: %w", err)
	}
	spec := &specEvidence{AffectedFiles: map[string]struct{}{}}
	inFrontmatter := false
	inAffected := false
	for lineNo, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if lineNo == 0 && line == "---" {
			inFrontmatter = true
			continue
		}
		if inFrontmatter {
			if line == "---" {
				inFrontmatter = false
				continue
			}
			if strings.HasPrefix(line, "status:") {
				spec.Status = strings.TrimSpace(strings.TrimPrefix(line, "status:"))
			}
			continue
		}
		if strings.HasPrefix(line, "## ") {
			inAffected = line == "## Affected files"
			continue
		}
		if inAffected && strings.HasPrefix(line, "- ") {
			item := strings.TrimSpace(strings.TrimPrefix(line, "- "))
			item = strings.Trim(item, "`")
			if rel, err := cleanRepoPath("affected file", item); err == nil {
				spec.AffectedFiles[rel] = struct{}{}
			}
		}
	}
	return spec, nil
}

func (s *specEvidence) Allows(rel string) bool {
	_, ok := s.AffectedFiles[rel]
	return ok
}

func readTaskRecord(root, rel string) (*contract.TaskRecordV1, error) {
	if err := ensurePathReadableInside(root, rel); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return nil, fmt.Errorf("read task record: %w", err)
	}
	var record contract.TaskRecordV1
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&record); err != nil {
		return nil, fmt.Errorf("task record JSON: %w", err)
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("task record JSON: multiple JSON values")
		}
		return nil, fmt.Errorf("task record JSON: %w", err)
	}
	return &record, nil
}

func changedFiles(ctx context.Context, root string, opts ValidateOptions, deps ValidateDependencies) ([]string, error) {
	seen := map[string]struct{}{}
	for _, value := range opts.ChangedFiles {
		rel, err := cleanRepoPath("--changed-file", value)
		if err != nil {
			return nil, err
		}
		seen[rel] = struct{}{}
	}
	if opts.Base != "" {
		baseRevision, err := resolveGitRevision(ctx, root, opts.Base, deps)
		if err != nil {
			return nil, fmt.Errorf("resolve --base: %w", err)
		}
		out, err := deps.RunGit(ctx, root, "diff", "--name-only", "--diff-filter=ACDMRTUXB", baseRevision, "HEAD", "--")
		if err != nil {
			return nil, fmt.Errorf("git diff --name-only: %w", err)
		}
		for _, rel := range strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n") {
			rel, err := cleanRepoPath("changed path", rel)
			if err == nil {
				seen[rel] = struct{}{}
			}
		}
	}
	status, err := deps.RunGit(ctx, root, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err == nil {
		for _, rel := range parseGitStatusPaths(status) {
			seen[rel] = struct{}{}
		}
	} else if opts.Base != "" {
		return nil, fmt.Errorf("git status: %w", err)
	}
	changed := make([]string, 0, len(seen))
	for rel := range seen {
		changed = append(changed, rel)
	}
	sort.Strings(changed)
	return changed, nil
}

func parseGitStatusPaths(data []byte) []string {
	parts := bytes.Split(data, []byte{0})
	var paths []string
	for i := 0; i < len(parts); i++ {
		part := string(parts[i])
		if len(part) < 4 {
			continue
		}
		code := part[:2]
		name := strings.TrimSpace(part[3:])
		if code[0] == 'R' || code[0] == 'C' {
			i++
			if i < len(parts) {
				name = strings.TrimSpace(string(parts[i]))
			}
		}
		if rel, err := cleanRepoPath("changed path", name); err == nil {
			paths = append(paths, rel)
		}
	}
	return paths
}

func validateCheckEvidence(ctx context.Context, deps ValidateDependencies, result *ValidateResult, checks []contract.TaskCheckRecord, currentRevision, digest string) {
	passed := false
	for i, check := range checks {
		if check.Status != contract.CheckStatusPassed {
			continue
		}
		passed = true
		if check.Revision == "" {
			result.Failures = append(result.Failures, fmt.Sprintf("task record checks[%d].revision is required", i))
		} else if resolved, err := resolveGitRevision(ctx, result.ScopeRoot, check.Revision, deps); err == nil && currentRevision != "" && resolved != currentRevision {
			result.Failures = append(result.Failures, fmt.Sprintf("task record checks[%d].revision is stale", i))
		} else if err != nil && check.Revision != currentRevision {
			result.Failures = append(result.Failures, fmt.Sprintf("task record checks[%d].revision is stale", i))
		}
		if check.ContentDigest == "" {
			result.Failures = append(result.Failures, fmt.Sprintf("task record checks[%d].content_digest is required", i))
		} else if check.ContentDigest != digest {
			result.Failures = append(result.Failures, fmt.Sprintf("task record checks[%d].content_digest is stale", i))
		}
	}
	if !passed {
		result.Failures = append(result.Failures, "task record requires at least one passed check")
	}
}

func validateReviewEvidence(ctx context.Context, deps ValidateDependencies, result *ValidateResult, review *contract.TaskReviewRecord, currentRevision, digest string) {
	if review == nil {
		result.Failures = append(result.Failures, "high-risk validation requires independent review evidence")
		return
	}
	if review.Kind != contract.ReviewKindIndependent {
		result.Failures = append(result.Failures, "review.kind must be independent")
	}
	if strings.TrimSpace(review.Evidence) == "" {
		result.Failures = append(result.Failures, "review.evidence is required")
	}
	if review.Revision == "" {
		result.Failures = append(result.Failures, "review.revision is required")
	} else if resolved, err := resolveGitRevision(ctx, result.ScopeRoot, review.Revision, deps); err == nil && currentRevision != "" && resolved != currentRevision {
		result.Failures = append(result.Failures, "review.revision is stale")
	} else if err != nil && review.Revision != currentRevision {
		result.Failures = append(result.Failures, "review.revision is stale")
	}
	if review.ContentDigest == "" {
		result.Failures = append(result.Failures, "review.content_digest is required")
	} else if review.ContentDigest != digest {
		result.Failures = append(result.Failures, "review.content_digest is stale")
	}
}

func digestOwnedFiles(root string, files []string) (string, error) {
	entries := make([]contract.TaskContentEntry, 0, len(files))
	for _, file := range files {
		rel, err := cleanRepoPath("owned file", file)
		if err != nil {
			return "", err
		}
		abs := filepath.Join(root, filepath.FromSlash(rel))
		info, err := os.Lstat(abs)
		if err != nil {
			if os.IsNotExist(err) {
				entries = append(entries, contract.TaskContentEntry{Path: rel, Mode: "missing", Missing: true})
				continue
			}
			return "", fmt.Errorf("inspect %s: %w", rel, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("owned file %s is a symlink", rel)
		}
		if info.IsDir() {
			return "", fmt.Errorf("owned file %s is a directory", rel)
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			return "", fmt.Errorf("read %s: %w", rel, err)
		}
		entries = append(entries, contract.TaskContentEntry{
			Path:    rel,
			Mode:    fmt.Sprintf("%04o", info.Mode().Perm()),
			Content: data,
		})
	}
	return contract.DigestTaskContent(entries), nil
}

func validateOwnedScope(root, rel string) error {
	if err := ensurePathInside(root, rel); err != nil {
		return err
	}
	owner := nearestAIDLCScope(root, rel)
	if owner != root {
		return fmt.Errorf("changed file %s is owned by nested AIDLC scope %s", rel, owner)
	}
	return nil
}

func nearestAIDLCScope(root, rel string) string {
	abs := filepath.Join(root, filepath.FromSlash(rel))
	dir := abs
	if info, err := os.Lstat(abs); err == nil && !info.IsDir() {
		dir = filepath.Dir(abs)
	} else if filepath.Ext(abs) != "" {
		dir = filepath.Dir(abs)
	}
	root = filepath.Clean(root)
	for {
		if isAIDLCScope(dir) {
			return filepath.Clean(dir)
		}
		if dir == root {
			return root
		}
		next := filepath.Dir(dir)
		if next == dir || !strings.HasPrefix(next, root) {
			return root
		}
		dir = next
	}
}

func isAIDLCScope(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, ".ai", "README.md")); err != nil {
		return false
	}
	if _, err := os.Stat(filepath.Join(dir, "docs", "spec", "README.md")); err != nil {
		return false
	}
	return true
}

func ensurePathReadableInside(root, rel string) error {
	if err := ensurePathInside(root, rel); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
		return fmt.Errorf("read %s: %w", rel, err)
	}
	return nil
}

func ensurePathInside(root, rel string) error {
	rel, err := cleanRepoPath("path", rel)
	if err != nil {
		return err
	}
	abs := filepath.Join(root, filepath.FromSlash(rel))
	if info, err := os.Lstat(abs); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("path %s is a symlink", rel)
	}
	resolved := abs
	if realPath, err := filepath.EvalSymlinks(abs); err == nil {
		resolved = realPath
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("resolve %s: %w", rel, err)
	}
	if !fileWithin(root, resolved) {
		return fmt.Errorf("path %s escapes scope root", rel)
	}
	return nil
}

func cleanRepoPath(kind, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("%s must not be empty", kind)
	}
	if strings.HasPrefix(value, "/") || strings.Contains(value, "\\") || strings.Contains(value, ":") {
		return "", fmt.Errorf("%s %q must be slash-relative", kind, value)
	}
	value = filepath.ToSlash(value)
	for _, segment := range strings.Split(value, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return "", fmt.Errorf("%s %q must not contain empty, current, or parent path segments", kind, value)
		}
	}
	cleaned := path.Clean(value)
	if cleaned == "." || strings.HasPrefix(cleaned, "../") || cleaned == ".." {
		return "", fmt.Errorf("%s %q must not traverse parents", kind, value)
	}
	return cleaned, nil
}

func resolveScopeRoot(dir string) (string, error) {
	if strings.TrimSpace(dir) == "" {
		return "", fmt.Errorf("--dir must not be empty")
	}
	root, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolve --dir: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return "", fmt.Errorf("inspect --dir: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("--dir is not a directory: %s", root)
	}
	return root, nil
}

func fileWithin(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != "..")
}

func pathSet(paths []string) map[string]struct{} {
	set := make(map[string]struct{}, len(paths))
	for _, value := range paths {
		if rel, err := cleanRepoPath("path", value); err == nil {
			set[rel] = struct{}{}
		}
	}
	return set
}

func resolveGitRevision(ctx context.Context, root, rev string, deps ValidateDependencies) (string, error) {
	deps = validateDependenciesWithDefaults(deps)
	rev = strings.TrimSpace(rev)
	if rev == "" || strings.HasPrefix(rev, "-") || strings.ContainsAny(rev, "\x00\r\n") {
		return "", fmt.Errorf("invalid revision %q", rev)
	}
	out, err := deps.RunGit(ctx, root, "rev-parse", "--verify", rev+"^{commit}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func validateDependenciesWithDefaults(deps ValidateDependencies) ValidateDependencies {
	if deps.RunGit == nil {
		deps.RunGit = func(ctx context.Context, dir string, args ...string) ([]byte, error) {
			cmd := exec.CommandContext(ctx, "git", args...)
			cmd.Dir = dir
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			out, err := cmd.Output()
			if err != nil {
				msg := strings.TrimSpace(stderr.String())
				if msg != "" {
					return out, fmt.Errorf("%w: %s", err, msg)
				}
				return out, err
			}
			return out, nil
		}
	}
	return deps
}

func printValidateResult(w io.Writer, result ValidateResult, format string) error {
	if strings.EqualFold(format, "json") {
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	}
	if result.Passed {
		fmt.Fprintln(w, "aidlc validate: pass")
	} else {
		fmt.Fprintln(w, "aidlc validate: fail")
	}
	if result.Revision != "" {
		fmt.Fprintf(w, "revision: %s\n", result.Revision)
	}
	if result.ContentDigest != "" {
		fmt.Fprintf(w, "content digest: %s\n", result.ContentDigest)
	}
	for _, changed := range result.ChangedFiles {
		fmt.Fprintf(w, "changed: %s\n", changed)
	}
	for _, failure := range result.Failures {
		fmt.Fprintf(w, "failure: %s\n", failure)
	}
	for _, note := range result.Notes {
		fmt.Fprintf(w, "note: %s\n", note)
	}
	return nil
}

func printValidateUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: aidlc validate [--dir DIR] [--spec PATH] [--task-record PATH] [--base REV] [--changed-file PATH ...] [--high-risk] [--format text|json]")
}
