//go:build ignore

package main

import (
	"fmt"
	"os"

	"github.com/{{repo_slug}}/internal/{{package_name}}"
)

func main() {
	if err := {{package_name}}.Run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
