package {{package_name}}

import (
	"strings"
	"testing"
)

func TestGreet(t *testing.T) {
	msg := Greet("Cooper")
	if !strings.Contains(msg, "Cooper") {
		t.Fatalf("expected greeting to contain Cooper, got %q", msg)
	}
	if !strings.Contains(msg, "{{repo_name}}") {
		t.Fatalf("expected greeting to contain repo name, got %q", msg)
	}

	defaultMsg := Greet("")
	if !strings.Contains(defaultMsg, "world") {
		t.Fatalf("expected default greeting to contain world, got %q", defaultMsg)
	}
}

func TestRun(t *testing.T) {
	if err := Run(nil); err != nil {
		t.Fatalf("expected nil error for default Run, got %v", err)
	}
	if err := Run([]string{"developer"}); err != nil {
		t.Fatalf("expected nil error for Run with arg, got %v", err)
	}
}
