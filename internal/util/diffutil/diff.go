package diffutil

import (
	"fmt"
	"strings"

	"flux/internal/types/snapshot"

	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
)

// ComputeFileHunks uses gotextdiff to compute structured diff hunks.
func ComputeFileHunks(oldContent, newContent []byte) []snapshot.DiffHunk {
	oldText := string(oldContent)
	newText := string(newContent)

	edits := myers.ComputeEdits("a", oldText, newText)
	unified := gotextdiff.ToUnified("a", "b", oldText, edits)

	lines := strings.Split(fmt.Sprint(unified), "\n")
	return ParseUnifiedDiffLines(lines)
}

// ParseUnifiedDiffLines converts unified diff text lines into structured DiffHunks.
func ParseUnifiedDiffLines(lines []string) []snapshot.DiffHunk {
	var hunks []snapshot.DiffHunk
	var current *snapshot.DiffHunk

	for _, line := range lines {
		if strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++") || line == "" {
			continue
		}

		if strings.HasPrefix(line, "@@") {
			if current != nil {
				hunks = append(hunks, *current)
			}
			current = &snapshot.DiffHunk{}
			continue
		}

		if current == nil || len(line) == 0 {
			continue
		}

		switch line[0] {
		case '+':
			current.Lines = append(current.Lines, snapshot.DiffLine{
				Type:    snapshot.LineAdded,
				Content: line[1:],
			})
			current.NewCount++
		case '-':
			current.Lines = append(current.Lines, snapshot.DiffLine{
				Type:    snapshot.LineDeleted,
				Content: line[1:],
			})
			current.OldCount++
		default:
			content := line
			if len(content) > 0 && content[0] == ' ' {
				content = content[1:]
			}
			current.Lines = append(current.Lines, snapshot.DiffLine{
				Type:    snapshot.LineContext,
				Content: content,
			})
			current.OldCount++
			current.NewCount++
		}
	}

	if current != nil {
		hunks = append(hunks, *current)
	}

	return hunks
}

// CountLineChanges counts added and deleted lines between two byte contents.
func CountLineChanges(oldContent, newContent []byte) (addLines, delLines int) {
	edits := myers.ComputeEdits("a", string(oldContent), string(newContent))
	unified := gotextdiff.ToUnified("a", "b", string(oldContent), edits)

	for _, line := range strings.Split(fmt.Sprint(unified), "\n") {
		if len(line) > 0 {
			if line[0] == '+' && !strings.HasPrefix(line, "+++") {
				addLines++
			} else if line[0] == '-' && !strings.HasPrefix(line, "---") {
				delLines++
			}
		}
	}
	return
}

// IsBinaryContent checks if content appears to be binary (contains null bytes).
func IsBinaryContent(content []byte) bool {
	if len(content) == 0 {
		return false
	}
	limit := 512
	if len(content) < limit {
		limit = len(content)
	}
	for i := 0; i < limit; i++ {
		if content[i] == 0 {
			return true
		}
	}
	return false
}

// ComputeConflictDiff computes a DiffResult for conflict display.
// For binary files, only sets IsBinary and size fields.
func ComputeConflictDiff(localContent, remoteContent []byte, filePath string) *snapshot.DiffResult {
	result := &snapshot.DiffResult{
		SourceName: "本地",
		TargetName: "远端",
		HasDiff:    true,
	}

	if IsBinaryContent(localContent) || IsBinaryContent(remoteContent) {
		result.Files = []snapshot.DiffFileChange{
			{
				Path:     filePath,
				Status:   snapshot.FileModified,
				IsBinary: true,
				OldSize:  int64(len(localContent)),
				NewSize:  int64(len(remoteContent)),
			},
		}
		return result
	}

	hunks := ComputeFileHunks(localContent, remoteContent)
	addLines, delLines := CountLineChanges(localContent, remoteContent)

	result.Files = []snapshot.DiffFileChange{
		{
			Path:     filePath,
			Status:   snapshot.FileModified,
			IsBinary: false,
			OldSize:  int64(len(localContent)),
			NewSize:  int64(len(remoteContent)),
			AddLines: addLines,
			DelLines: delLines,
			Hunks:    hunks,
		},
	}

	return result
}
