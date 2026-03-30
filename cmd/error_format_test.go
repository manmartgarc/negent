package cmd

import (
	"errors"
	"strings"
	"testing"

	gitbackend "github.com/manmart/negent/internal/backend/git"
)

func TestFormatSyncOpErrorConflictWithFiles(t *testing.T) {
	err := formatSyncOpError("push", "negent push", &gitbackend.ConflictError{
		Command: "pull --rebase",
		Files:   []string{"a.txt", "b.txt"},
		Err:     errors.New("conflict"),
	})

	msg := err.Error()
	if !strings.Contains(msg, "push failed due to git content conflicts") {
		t.Fatalf("unexpected message: %q", msg)
	}
	if !strings.Contains(msg, "during pull --rebase") {
		t.Fatalf("expected command context in message: %q", msg)
	}
	if !strings.Contains(msg, "a.txt, b.txt") {
		t.Fatalf("expected conflicted files in message: %q", msg)
	}
	if !strings.Contains(msg, "negent push") {
		t.Fatalf("expected retry command guidance in message: %q", msg)
	}
}

func TestFormatSyncOpErrorGeneric(t *testing.T) {
	err := formatSyncOpError("pull", "negent pull", errors.New("network down"))
	msg := err.Error()
	if !strings.Contains(msg, "pull failed: network down") {
		t.Fatalf("unexpected message: %q", msg)
	}
}
