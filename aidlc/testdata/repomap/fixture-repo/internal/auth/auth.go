package auth

import "github.com/example/fixture/internal/core"

func Authorize(name string) string {
	return core.Greet(name)
}
