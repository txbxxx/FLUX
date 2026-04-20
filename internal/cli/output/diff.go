package output

import (
	"fmt"
	"io"
	"strings"

	typesSnapshot "flux/internal/types/snapshot"

	"github.com/charmbracelet/lipgloss"
)

// Diff color styles
var (
	DiffAddedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("2")) // green
	DiffDeletedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("1")) // red
	DiffModifiedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3")) // yellow
	diffHeaderStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("6")) // cyan
	diffFilePathStyle = lipgloss.NewStyle().Bold(true)
	diffStatStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241")) // dim gray
)

// RenderDiffSummary renders a git-diff --stat style summary of diff results.
func RenderDiffSummary(w io.Writer, result *typesSnapshot.DiffResult, useColor bool) {
	if !result.HasDiff {
		fmt.Fprintln(w, "无差异")
		return
	}

	// Header
	if useColor {
		fmt.Fprintf(w, "%s\n", diffHeaderStyle.Render(fmt.Sprintf("对比: %s → %s", result.SourceName, result.TargetName)))
	} else {
		fmt.Fprintf(w, "对比: %s → %s\n", result.SourceName, result.TargetName)
	}

	// File list
	for _, file := range result.Files {
		status := string(file.Status)
		path := file.Path

		if useColor {
			switch file.Status {
			case typesSnapshot.FileAdded:
				path = DiffAddedStyle.Render(file.Path)
				status = DiffAddedStyle.Render("added")
			case typesSnapshot.FileDeleted:
				path = DiffDeletedStyle.Render(file.Path)
				status = DiffDeletedStyle.Render("deleted")
			case typesSnapshot.FileModified:
				path = diffFilePathStyle.Render(file.Path)
				status = "modified"
			}
		}

		if file.IsBinary {
			fmt.Fprintf(w, "  %s  [%s] (binary, %d → %d bytes)\n", path, status, file.OldSize, file.NewSize)
		} else {
			stat := ""
			if file.AddLines > 0 || file.DelLines > 0 {
				stat = fmt.Sprintf(" (+%d/-%d)", file.AddLines, file.DelLines)
			}
			if useColor {
				fmt.Fprintf(w, "  %s  [%s]%s\n", path, status, diffStatStyle.Render(stat))
			} else {
				fmt.Fprintf(w, "  %s  [%s]%s\n", path, status, stat)
			}
		}
	}

	// Summary line
	stats := result.Stats
	summary := fmt.Sprintf("\n%d files changed: %d added, %d modified, %d deleted (+%d/-%d lines)",
		stats.TotalFiles, stats.AddedFiles, stats.ModifiedFiles, stats.DeletedFiles,
		stats.AddLines, stats.DelLines)

	if useColor {
		fmt.Fprintln(w, diffStatStyle.Render(summary))
	} else {
		fmt.Fprintln(w, summary)
	}

	// Partial mode warning
	if result.Partial {
		warning := fmt.Sprintf("注意: %s", result.PartialReason)
		if useColor {
			fmt.Fprintln(w, DiffDeletedStyle.Render(warning))
		} else {
			fmt.Fprintln(w, warning)
		}
	}
}

// RenderUnifiedDiff renders a unified diff output with colored lines.
// When hideGitHeader is true, the "diff --git a/... b/..." header line is omitted.
func RenderUnifiedDiff(w io.Writer, result *typesSnapshot.DiffResult, useColor bool, hideGitHeader bool) {
	if !result.HasDiff {
		fmt.Fprintln(w, "无差异")
		return
	}

	// Header
	if useColor {
		fmt.Fprintf(w, "%s\n", diffHeaderStyle.Render(fmt.Sprintf("--- %s", result.SourceName)))
		fmt.Fprintf(w, "%s\n", diffHeaderStyle.Render(fmt.Sprintf("+++ %s", result.TargetName)))
	} else {
		fmt.Fprintf(w, "--- %s\n+++ %s\n", result.SourceName, result.TargetName)
	}

	for _, file := range result.Files {
		// File header
		fmt.Fprintf(w, "\n")
		if !hideGitHeader {
			if useColor {
				fmt.Fprintf(w, "%s\n", diffFilePathStyle.Render(fmt.Sprintf("diff --git a/%s b/%s", file.Path, file.Path)))
			} else {
				fmt.Fprintf(w, "diff --git a/%s b/%s\n", file.Path, file.Path)
			}
		}

		if file.IsBinary {
			fmt.Fprintln(w, "  Binary files differ")
			continue
		}

		// Status-only files (added/deleted without hunks)
		if len(file.Hunks) == 0 {
			switch file.Status {
			case typesSnapshot.FileAdded:
				fmt.Fprintf(w, "  new file: %s\n", file.Path)
			case typesSnapshot.FileDeleted:
				fmt.Fprintf(w, "  deleted file: %s\n", file.Path)
			default:
				fmt.Fprintf(w, "  %s\n", file.Path)
			}
			continue
		}

		// Render hunks
		for _, hunk := range file.Hunks {
			header := fmt.Sprintf("@@ -%d,%d +%d,%d @@", hunk.OldStart, hunk.OldCount, hunk.NewStart, hunk.NewCount)
			if useColor {
				fmt.Fprintf(w, "%s\n", diffHeaderStyle.Render(header))
			} else {
				fmt.Fprintf(w, "%s\n", header)
			}

			for _, line := range hunk.Lines {
				switch line.Type {
				case typesSnapshot.LineAdded:
					if useColor {
						fmt.Fprintf(w, "%s\n", DiffAddedStyle.Render("+"+line.Content))
					} else {
						fmt.Fprintf(w, "+%s\n", line.Content)
					}
				case typesSnapshot.LineDeleted:
					if useColor {
						fmt.Fprintf(w, "%s\n", DiffDeletedStyle.Render("-"+line.Content))
					} else {
						fmt.Fprintf(w, "-%s\n", line.Content)
					}
				default:
					fmt.Fprintf(w, " %s\n", line.Content)
				}
			}
		}
	}
}

// RenderSideBySideDiff renders a side-by-side diff view.
func RenderSideBySideDiff(w io.Writer, result *typesSnapshot.DiffResult, useColor bool) {
	if !result.HasDiff {
		fmt.Fprintln(w, "无差异")
		return
	}

	// Get terminal width, default 120
	width := getTerminalWidth()
	if width < 80 {
		fmt.Fprintln(w, "终端宽度不足 80 列，无法并排显示。请使用 -v 代替 --side-by-side")
		return
	}

	halfWidth := (width - 3) / 2 // 3 for separator " | "

	// Header
	fmt.Fprintf(w, "%s | %s\n",
		padOrTruncate(fmt.Sprintf("--- %s", result.SourceName), halfWidth),
		padOrTruncate(fmt.Sprintf("+++ %s", result.TargetName), halfWidth))
	fmt.Fprintf(w, "%s-+-%s\n", strings.Repeat("-", halfWidth), strings.Repeat("-", halfWidth))

	for _, file := range result.Files {
		fmt.Fprintf(w, "\n%s\n", padOrTruncate(file.Path, width))

		if file.IsBinary {
			fmt.Fprintf(w, "%s\n", padOrTruncate("  [Binary files differ]", width))
			continue
		}

		for _, hunk := range file.Hunks {
			for _, line := range hunk.Lines {
				switch line.Type {
				case typesSnapshot.LineAdded:
					left := padOrTruncate("", halfWidth)
					right := "+" + line.Content
					if useColor {
						right = DiffAddedStyle.Render(padOrTruncate(right, halfWidth))
					} else {
						right = padOrTruncate(right, halfWidth)
					}
					fmt.Fprintf(w, "%s | %s\n", left, right)
				case typesSnapshot.LineDeleted:
					left := "-" + line.Content
					if useColor {
						left = DiffDeletedStyle.Render(padOrTruncate(left, halfWidth))
					} else {
						left = padOrTruncate(left, halfWidth)
					}
					right := padOrTruncate("", halfWidth)
					fmt.Fprintf(w, "%s | %s\n", left, right)
				default:
					content := " " + line.Content
					fmt.Fprintf(w, "%s | %s\n", padOrTruncate(content, halfWidth), padOrTruncate(content, halfWidth))
				}
			}
		}
	}
}

// getTerminalWidth attempts to detect the terminal width.
func getTerminalWidth() int {
	// Default width if detection fails
	return 120
}

// padOrTruncate pads a string to exactly n display width, or truncates it.
func padOrTruncate(s string, n int) string {
	w := DisplayWidth(s)
	if w >= n {
		// Truncate
		truncated := ""
		tw := 0
		for _, r := range s {
			rw := 1
			if r > 0x7F {
				rw = 2
			}
			if tw+rw > n-1 {
				break
			}
			truncated += string(r)
			tw += rw
		}
		return truncated + "~"
	}
	return s + strings.Repeat(" ", n-w)
}
