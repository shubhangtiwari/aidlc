package generator

import "github.com/shubhangtiwari/aidlc/aidlc/internal/contract"

const readmeInitMarker = "<!-- INIT:BEGIN -->"

type Options struct {
	TargetDir string
	IDE       contract.IDE
	IDEs      []contract.IDE
}

type Result struct {
	Written     []string
	Projections []contract.GeneratedProjection
}

type document struct {
	Name        string
	Description string
	Body        string
	Raw         []byte
}

type modelDefault struct {
	Model     string
	Reasoning string
	Effort    string
}

type sourceData struct {
	Facts         ProjectFacts
	Personas      []document
	Skills        []document
	Installed     []installedSkill
	ModelDefaults map[string]map[string]modelDefault
	SharedBody    string
}

type installedSkill struct {
	Name        string
	Description string
	Source      string
	Dirs        []string
	Files       []installedFile
}

type installedFile struct {
	Path    string
	Content []byte
	Mode    string
}

type ProjectFacts struct {
	HasManifest  bool
	ProjectName  string
	Language     string
	ManifestPath string
	SourceRoot   string
	PackageName  string
	Runtime      string
	BuildTool    string
	ModulePath   string
}
