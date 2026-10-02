// Command log-analyzer analyzes Apache access logs of REST APIs.
// See docs/SPEC.md for the full specification.
package main

import (
	"os"

	"log-analyzer/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
