package core

import "strings"

func Greet(name string) string {
	return "hello " + strings.TrimSpace(name)
}
