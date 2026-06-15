package auth

import "github.com/example/fixture/internal/core"

type SessionPolicy struct {
	AllowedRole string
}

func Authorize(name string) string {
	return core.Greet(NormalizePrincipal(name))
}

func NormalizePrincipal(name string) string {
	return core.NormalizeGreetingName(name)
}
