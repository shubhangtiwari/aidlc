package search

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/repomap/model"
)

type ExactSearcher struct {
	RgPath string
}

var _ model.ExactSearcher = (*ExactSearcher)(nil)

func NewExactSearcher() *ExactSearcher {
	return &ExactSearcher{}
}

func (s *ExactSearcher) SearchExact(ctx context.Context, request model.ExactSearchRequest) (model.ExactSearchResponse, error) {
	plan, err := (model.ExactSearchPlan{
		Enabled:    true,
		Literals:   request.Literals,
		Paths:      request.Paths,
		MaxResults: request.MaxResults,
		MaxBytes:   request.MaxBytes,
		TimeoutMS:  request.TimeoutMS,
	}).Normalize()
	if err != nil {
		return model.ExactSearchResponse{}, err
	}
	if len(plan.Literals) == 0 || plan.MaxResults <= 0 {
		return model.ExactSearchResponse{}, nil
	}
	root, err := filepath.Abs(request.Root)
	if err != nil {
		return model.ExactSearchResponse{}, fmt.Errorf("resolve root: %w", err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return model.ExactSearchResponse{}, fmt.Errorf("resolve root symlink: %w", err)
	}
	rgPath := s.RgPath
	if strings.TrimSpace(rgPath) == "" {
		rgPath, err = exec.LookPath("rg")
		if err != nil {
			return model.ExactSearchResponse{Diagnostics: []string{"rg unavailable; exact search skipped"}}, nil
		}
	} else if _, err := os.Stat(rgPath); err != nil {
		if os.IsNotExist(err) {
			return model.ExactSearchResponse{Diagnostics: []string{"rg unavailable; exact search skipped"}}, nil
		}
		return model.ExactSearchResponse{}, fmt.Errorf("stat rg: %w", err)
	}
	paths, diagnostics := validatedSearchPaths(root, plan.Paths)
	if len(paths) == 0 {
		paths = []string{"."}
	}

	timeout := time.Duration(plan.TimeoutMS) * time.Millisecond
	searchCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var response model.ExactSearchResponse
	response.Diagnostics = append(response.Diagnostics, diagnostics...)
	remainingBytes := plan.MaxBytes
	for _, literal := range plan.Literals {
		if len(response.Results) >= plan.MaxResults || remainingBytes <= 0 {
			response.Truncated = true
			break
		}
		results, usedBytes, truncated, err := runRG(searchCtx, root, rgPath, literal, paths, plan.MaxResults-len(response.Results), remainingBytes)
		remainingBytes -= usedBytes
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) || searchCtx.Err() != nil {
				response.Diagnostics = append(response.Diagnostics, "exact search timed out")
				response.Truncated = true
				return response, nil
			}
			response.Diagnostics = append(response.Diagnostics, fmt.Sprintf("rg failed for literal %q: %v", literal, err))
			continue
		}
		response.Results = append(response.Results, results...)
		if truncated {
			response.Truncated = true
		}
	}
	return response, nil
}

func validatedSearchPaths(root string, hints []string) ([]string, []string) {
	var paths []string
	var diagnostics []string
	for _, hint := range hints {
		joined := filepath.Join(root, filepath.FromSlash(hint))
		resolved := joined
		if realPath, err := filepath.EvalSymlinks(joined); err == nil {
			resolved = realPath
		} else if !os.IsNotExist(err) {
			diagnostics = append(diagnostics, fmt.Sprintf("exact search path %q unavailable: %v", hint, err))
			continue
		}
		if !isWithinRoot(root, resolved) {
			diagnostics = append(diagnostics, fmt.Sprintf("exact search path %q escapes repository root; skipped", hint))
			continue
		}
		paths = append(paths, hint)
	}
	return paths, diagnostics
}

func isWithinRoot(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != "..")
}

func runRG(ctx context.Context, root, rgPath, literal string, paths []string, maxResults, maxBytes int) ([]model.ExactSearchResult, int, bool, error) {
	args := []string{
		"--fixed-strings",
		"--line-number",
		"--column",
		"--no-heading",
		"--color", "never",
		"--with-filename",
		"--field-match-separator", "\t",
		"--max-count", strconv.Itoa(maxResults),
		"--regexp", literal,
		"--",
	}
	args = append(args, paths...)
	cmd := exec.CommandContext(ctx, rgPath, args...)
	cmd.Dir = root
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, 0, false, err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, 0, false, err
	}
	limited, err := io.ReadAll(io.LimitReader(stdout, int64(maxBytes+1)))
	waitErr := cmd.Wait()
	if err != nil {
		return nil, len(limited), len(limited) > maxBytes, err
	}
	truncated := len(limited) > maxBytes
	if truncated {
		limited = limited[:maxBytes]
	}
	if waitErr != nil {
		if exit, ok := waitErr.(*exec.ExitError); ok && exit.ExitCode() == 1 {
			return nil, len(limited), truncated, nil
		}
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = waitErr.Error()
		}
		return nil, len(limited), truncated, errors.New(message)
	}
	results := parseRGOutput(limited)
	if len(results) > maxResults {
		results = results[:maxResults]
		truncated = true
	}
	return results, len(limited), truncated, nil
}

func parseRGOutput(output []byte) []model.ExactSearchResult {
	var results []model.ExactSearchResult
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		fields := strings.SplitN(scanner.Text(), "\t", 4)
		if len(fields) < 4 {
			continue
		}
		line, _ := strconv.Atoi(fields[1])
		column, _ := strconv.Atoi(fields[2])
		results = append(results, model.ExactSearchResult{
			Path:    filepath.ToSlash(fields[0]),
			Line:    line,
			Column:  column,
			Match:   fields[3],
			Snippet: fields[3],
		})
	}
	return results
}
