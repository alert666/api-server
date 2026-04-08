package main

import (
	"fmt"
	"os"

	"github.com/alert666/api-server/cmd"
)

// @title           Swagger API
// @version         1.0
// @description     api-server api docs.
// @host      0.0.0.0:8080
func main() {
	if err := cmd.NewCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
