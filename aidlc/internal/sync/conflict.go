package sync

type DecisionState string

const (
	StateCreate          DecisionState = "create"
	StateSkip            DecisionState = "skip"
	StateUpdateClean     DecisionState = "update-clean"
	StateOverwrite       DecisionState = "overwrite"
	StateConflict        DecisionState = "conflict"
	StateRemovedUpstream DecisionState = "removed-upstream"
)

type Mode string

const (
	ModeInit   Mode = "init"
	ModeUpdate Mode = "update"
)

type Decision struct {
	Path             string
	State            DecisionState
	Reason           string
	PreviousChecksum string
	LocalChecksum    string
	UpstreamChecksum string
	Mode             string
}

func (d Decision) IsWritable() bool {
	return d.State == StateCreate || d.State == StateUpdateClean || d.State == StateOverwrite
}
