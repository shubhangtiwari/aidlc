package auth

import "testing"

func TestAuthorize(t *testing.T) {
	if got := Authorize("agent"); got != "hello agent" {
		t.Fatalf("Authorize() = %q", got)
	}
}
