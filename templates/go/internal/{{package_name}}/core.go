//go:build ignore

// Package {{package_name}} provides the core domain logic for {{repo_name}}.
package {{package_name}}

import "fmt"

// Greet returns a greeting message for {{repo_name}}.
func Greet(name string) string {
	if name == "" {
		name = "world"
	}
	return fmt.Sprintf("Hello, %s from {{repo_name}}!", name)
}

// Run executes the primary CLI workflow.
func Run(args []string) error {
	target := "world"
	if len(args) > 0 && args[0] != "" {
		target = args[0]
	}
	fmt.Println(Greet(target))
	return nil
}
