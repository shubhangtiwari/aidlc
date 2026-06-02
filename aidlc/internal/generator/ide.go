package generator

import (
	"fmt"

	"github.com/aidlc/ai-dlc-template/aidlc/internal/contract"
)

func selectedIDEs(options Options) ([]contract.IDE, error) {
	if len(options.IDEs) > 0 {
		return explicitIDEs(options.IDEs)
	}
	return expandedIDEs(options.IDE)
}

func expandedIDEs(ide contract.IDE) ([]contract.IDE, error) {
	switch ide {
	case contract.IDEAll:
		return contract.ConcreteIDEs(), nil
	case contract.IDEClaude, contract.IDECodex, contract.IDECursor, contract.IDECopilot, contract.IDEWindsurf:
		return []contract.IDE{ide}, nil
	default:
		return nil, fmt.Errorf("unsupported IDE %q", ide)
	}
}

func explicitIDEs(values []contract.IDE) ([]contract.IDE, error) {
	selected := make(map[contract.IDE]bool, len(values))
	for _, value := range values {
		ide, err := contract.ParseIDE(value.String())
		if err != nil {
			return nil, err
		}
		if ide.IsAggregate() {
			return nil, fmt.Errorf("explicit IDE list cannot include aggregate IDE %q", ide)
		}
		selected[ide] = true
	}

	out := make([]contract.IDE, 0, len(selected))
	for _, ide := range contract.ConcreteIDEs() {
		if selected[ide] {
			out = append(out, ide)
		}
	}
	return out, nil
}

func markerIDE(ide contract.IDE) string {
	return ide.String()
}
