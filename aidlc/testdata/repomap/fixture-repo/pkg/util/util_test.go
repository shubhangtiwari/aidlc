package util

import "testing"

func TestIdentity(t *testing.T) {
	if got := Identity("agent"); got != "agent" {
		t.Fatalf("Identity() = %q", got)
	}
}
