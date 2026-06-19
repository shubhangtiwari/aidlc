package core

import "strings"

func Greet(name string) string {
	return "hello " + NormalizeGreetingName(name)
}

func NormalizeGreetingName(name string) string {
	return strings.TrimSpace(name)
}
