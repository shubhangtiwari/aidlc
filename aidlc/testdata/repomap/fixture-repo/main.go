package main

import (
	"fmt"

	"github.com/example/fixture/internal/auth"
)

func main() {
	fmt.Println(auth.Authorize("agent"))
}
