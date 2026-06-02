package source

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type GitHub struct {
	Owner      string
	Repo       string
	Ref        string
	HTTPClient *http.Client
}

func (g GitHub) Snapshot(ctx context.Context) (Snapshot, error) {
	if g.Owner == "" || g.Repo == "" {
		return Snapshot{}, fmt.Errorf("github owner and repo are required")
	}
	ref := g.Ref
	if ref == "" {
		ref = "main"
	}
	url := fmt.Sprintf("https://github.com/%s/%s/archive/%s.zip", g.Owner, g.Repo, ref)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Snapshot{}, err
	}
	client := g.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return Snapshot{}, fmt.Errorf("fetch github archive: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Snapshot{}, fmt.Errorf("fetch github archive: %s", resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return Snapshot{}, fmt.Errorf("read github archive: %w", err)
	}
	return Archive{
		Data:   data,
		Source: strings.TrimSuffix(url, ".zip"),
		Ref:    ref,
		Commit: resp.Header.Get("ETag"),
	}.Snapshot(ctx)
}
