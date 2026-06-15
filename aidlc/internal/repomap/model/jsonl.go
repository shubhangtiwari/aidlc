package model

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
)

type SortableRecord interface {
	SortKey() string
}

func WriteJSONL[T SortableRecord](w io.Writer, records []T) error {
	sorted := append([]T(nil), records...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].SortKey() < sorted[j].SortKey()
	})

	for _, record := range sorted {
		line, err := json.Marshal(record)
		if err != nil {
			return fmt.Errorf("marshal JSONL record: %w", err)
		}
		if bytes.ContainsAny(line, "\r\n") {
			return fmt.Errorf("marshal JSONL record: unexpected line break")
		}
		if _, err := w.Write(line); err != nil {
			return fmt.Errorf("write JSONL record: %w", err)
		}
		if _, err := io.WriteString(w, "\n"); err != nil {
			return fmt.Errorf("write JSONL newline: %w", err)
		}
	}
	return nil
}

func ReadJSONL[T any](r io.Reader) ([]T, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	var records []T
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var record T
		if err := json.Unmarshal(line, &record); err != nil {
			return nil, fmt.Errorf("read JSONL line %d: %w", lineNumber, err)
		}
		records = append(records, record)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan JSONL: %w", err)
	}
	return records, nil
}
