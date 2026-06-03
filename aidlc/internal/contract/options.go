package contract

type CommandName string

const (
	CommandInit    CommandName = "init"
	CommandUpdate  CommandName = "update"
	CommandUpgrade CommandName = "upgrade"
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

type UpgradeOptions struct {
	Repository string
	Version    UpgradeVersionSelector
	InstallDir string
	DryRun     bool
}

type UpgradeVersionSelector struct {
	Value    string
	Explicit bool
}

type SourceOptions struct {
	Kind string
	URL  string
	Ref  string
	Path string
}
