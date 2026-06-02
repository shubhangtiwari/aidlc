package contract

type CommandName string

const (
	CommandInit    CommandName = "init"
	CommandUpdate  CommandName = "update"
	CommandVersion CommandName = "version"
)

const (
	ExitOK       = 0
	ExitConflict = 1
	ExitUsage    = 2
)

type InitOptions struct {
	IDE       IDE
	TargetDir string
	Source    SourceOptions
	DryRun    bool
}

type UpdateOptions struct {
	TargetDir string
	Source    SourceOptions
	DryRun    bool
}

type SourceOptions struct {
	Kind string
	URL  string
	Ref  string
	Path string
}
