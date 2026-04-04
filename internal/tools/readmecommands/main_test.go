package main

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestCommandRows(t *testing.T) {
	root := &cobra.Command{Use: "negent"}
	addCmd := &cobra.Command{Use: "add <agent>", Short: "Add agent", Run: func(*cobra.Command, []string) {}}
	autoCmd := &cobra.Command{Use: "auto", Short: "Manage auto"}
	autoCmd.AddCommand(
		&cobra.Command{Use: "enable", Short: "Enable", Run: func(*cobra.Command, []string) {}},
		&cobra.Command{Use: "disable", Short: "Disable", Run: func(*cobra.Command, []string) {}},
	)
	configCmd := &cobra.Command{Use: "config", Short: "Manage config"}
	configCmd.AddCommand(&cobra.Command{Use: "edit", Short: "Edit config", Run: func(*cobra.Command, []string) {}})
	diffCmd := &cobra.Command{Use: "diff", Short: "Show diff", Run: func(*cobra.Command, []string) {}}
	root.AddCommand(addCmd, autoCmd, configCmd, diffCmd)
	root.InitDefaultHelpCmd()

	rows := commandRows(root)
	got := make([]string, 0, len(rows))
	for _, row := range rows {
		got = append(got, row.Command)
	}

	want := []string{
		"add <agent>",
		"auto",
		"config edit",
		"diff",
	}

	if len(got) != len(want) {
		t.Fatalf("unexpected row count: got %d want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected row at %d: got %q want %q", i, got[i], want[i])
		}
	}
}

func TestReplaceCommandTable(t *testing.T) {
	original := strings.Join([]string{
		"# negent",
		"",
		"## Command reference",
		"",
		"| Command | Purpose |",
		"| --- | --- |",
		"| `old` | stale |",
		"",
		"## Configuration",
		"content",
	}, "\n")

	rows := []commandRow{
		{Command: "add <agent>", Purpose: "Add agent"},
		{Command: "diff", Purpose: "Show diff"},
	}

	updated, changed, err := replaceCommandTable(original, rows)
	if err != nil {
		t.Fatalf("replaceCommandTable returned error: %v", err)
	}
	if !changed {
		t.Fatalf("expected replacement to report changed")
	}

	if !strings.Contains(updated, "| `add <agent>` | Add agent |") {
		t.Fatalf("updated table missing add row:\n%s", updated)
	}
	if !strings.Contains(updated, "| `diff` | Show diff |") {
		t.Fatalf("updated table missing diff row:\n%s", updated)
	}
	if strings.Contains(updated, "| `old` | stale |") {
		t.Fatalf("updated table still contains stale row:\n%s", updated)
	}
}

func TestReplaceCommandTableMissingSection(t *testing.T) {
	_, _, err := replaceCommandTable("# negent\n", nil)
	if err == nil {
		t.Fatalf("expected error for missing section")
	}
}
