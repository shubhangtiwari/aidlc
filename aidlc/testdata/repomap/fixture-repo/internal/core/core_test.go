package core

import "testing"

func TestGreet(t *testing.T) {
	if got := Greet(" agent "); got != "hello agent" {
		t.Fatalf("Greet() = %q", got)
	}
}
