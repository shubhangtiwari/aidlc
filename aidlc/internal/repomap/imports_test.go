package repomap

import "testing"

func TestExtractImportsCoversSupportedLanguages(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		content  string
		expected []string
	}{
		{
			name: "go",
			path: "main.go",
			content: `package main
import (
	"fmt"
	core "example.com/project/internal/core"
)
import "os"
`,
			expected: []string{"example.com/project/internal/core", "fmt", "os"},
		},
		{
			name:     "python",
			path:     "app.py",
			content:  "import os, sys\nfrom package.module import thing\n",
			expected: []string{"os", "package.module", "sys"},
		},
		{
			name:     "js",
			path:     "app.js",
			content:  "import React from 'react';\nimport './setup';\nconst fs = require('fs');\nexport {x} from '@scope/pkg';\n",
			expected: []string{"./setup", "@scope/pkg", "fs", "react"},
		},
		{
			name:     "java",
			path:     "App.java",
			content:  "import java.util.List;\nimport static org.junit.Assert.*;\n",
			expected: []string{"java.util.List", "org.junit.Assert.*"},
		},
		{
			name:     "rust",
			path:     "lib.rs",
			content:  "extern crate serde;\nuse std::collections::HashMap;\nuse crate::core::{Thing};\n",
			expected: []string{"crate::core::{Thing}", "serde", "std::collections::HashMap"},
		},
		{
			name:     "ruby",
			path:     "app.rb",
			content:  "require 'json'\nrequire_relative '../lib/core'\n",
			expected: []string{"../lib/core", "json"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractImports(tt.path, DetectLanguage(tt.path), tt.content)
			if len(got) != len(tt.expected) {
				t.Fatalf("ExtractImports() len = %d, want %d: %#v", len(got), len(tt.expected), got)
			}
			for i, want := range tt.expected {
				if got[i].ImportPath != want {
					t.Fatalf("ExtractImports()[%d] = %q, want %q", i, got[i].ImportPath, want)
				}
			}
		})
	}
}
