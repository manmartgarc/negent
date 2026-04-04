package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/manmart/negent/cmd"
)

const readmePath = "README.md"

type commandRow struct {
	Command string
	Purpose string
}

func main() {
	if err := updateREADME(readmePath); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func updateREADME(path string) error {
	root := cmd.RootCommand()
	root.InitDefaultCompletionCmd()

	rows := commandRows(root)

	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}

	updated, changed, err := replaceCommandTable(string(raw), rows)
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}

	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

func commandRows(root *cobra.Command) []commandRow {
	rows := make([]commandRow, 0)

	for _, top := range root.Commands() {
		if top.Hidden || top.Name() == "help" {
			continue
		}

		switch {
		case top.Runnable():
			rows = append(rows, rowFromCommand(root, top))
		case hasSingleRunnableChild(top):
			rows = append(rows, rowFromCommand(root, firstVisibleChild(top)))
		default:
			rows = append(rows, rowFromCommand(root, top))
		}
	}

	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Command < rows[j].Command
	})
	return rows
}

func hasSingleRunnableChild(c *cobra.Command) bool {
	visible := visibleChildren(c)
	return len(visible) == 1 && visible[0].Runnable()
}

func firstVisibleChild(c *cobra.Command) *cobra.Command {
	return visibleChildren(c)[0]
}

func visibleChildren(c *cobra.Command) []*cobra.Command {
	children := make([]*cobra.Command, 0)
	for _, child := range c.Commands() {
		if child.Hidden || child.Name() == "help" {
			continue
		}
		children = append(children, child)
	}
	return children
}

func rowFromCommand(root, c *cobra.Command) commandRow {
	return commandRow{
		Command: commandDisplay(root, c),
		Purpose: escapeMarkdownCell(strings.TrimSpace(c.Short)),
	}
}

func commandDisplay(root, c *cobra.Command) string {
	path := strings.TrimSpace(strings.TrimPrefix(c.CommandPath(), root.Name()))
	path = strings.TrimSpace(path)
	if path == "" {
		path = c.Name()
	}

	use := strings.TrimSpace(c.Use)
	parts := strings.Fields(use)
	if len(parts) <= 1 {
		return path
	}
	return strings.TrimSpace(path + " " + strings.Join(parts[1:], " "))
}

func replaceCommandTable(readme string, rows []commandRow) (string, bool, error) {
	lines := strings.Split(readme, "\n")
	sectionIndex := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "## Command reference" {
			sectionIndex = i
			break
		}
	}
	if sectionIndex == -1 {
		return "", false, fmt.Errorf("README command section not found")
	}

	tableStart := -1
	for i := sectionIndex + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			continue
		}
		if trimmed == "##" || strings.HasPrefix(trimmed, "## ") {
			break
		}
		if trimmed == "| Command | Purpose |" {
			tableStart = i
			break
		}
	}
	if tableStart == -1 {
		return "", false, fmt.Errorf("README command table header not found")
	}
	if tableStart+1 >= len(lines) || strings.TrimSpace(lines[tableStart+1]) != "| --- | --- |" {
		return "", false, fmt.Errorf("README command table separator not found")
	}

	tableEnd := tableStart + 2
	for tableEnd < len(lines) {
		if !strings.HasPrefix(strings.TrimSpace(lines[tableEnd]), "|") {
			break
		}
		tableEnd++
	}

	replacement := renderTable(rows)
	updatedLines := make([]string, 0, len(lines)-((tableEnd-tableStart)-len(replacement)))
	updatedLines = append(updatedLines, lines[:tableStart]...)
	updatedLines = append(updatedLines, replacement...)
	updatedLines = append(updatedLines, lines[tableEnd:]...)
	updated := strings.Join(updatedLines, "\n")
	return updated, updated != readme, nil
}

func renderTable(rows []commandRow) []string {
	lines := []string{
		"| Command | Purpose |",
		"| --- | --- |",
	}
	for _, row := range rows {
		lines = append(lines, fmt.Sprintf("| `%s` | %s |", escapeInlineCode(row.Command), row.Purpose))
	}
	return lines
}

func escapeInlineCode(s string) string {
	return strings.ReplaceAll(s, "`", "\\`")
}

func escapeMarkdownCell(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}
