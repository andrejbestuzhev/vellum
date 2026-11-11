package typesense

import (
	"strings"

	"github.com/freema/vellum/internal/vault"
)

// Section is one indexing unit of a note: the text before its first "## "
// heading (Heading empty), or one "## " section with everything under it up
// to the next "## " heading. Deeper headings stay inside their section.
type Section struct {
	Heading string
	Content string
	Line    int // 1-based file line where the section starts (frontmatter counted)
}

// SplitSections cuts a note into sections. content is the whole file, body
// the part after the frontmatter (as vault.Note has them). A preamble that
// is only blank lines is left out, unless the note has no "## " sections at
// all: every note yields at least one section, so it stays findable by title.
func SplitSections(content, body string) []Section {
	lines := vault.SplitLines(content)
	first := len(vault.SplitLines(content[:len(content)-len(body)])) // body's first line index
	var starts []vault.Heading
	for _, h := range vault.Outline(content) {
		if h.Level == 2 {
			starts = append(starts, h)
		}
	}

	var out []Section
	preEnd := len(lines)
	if len(starts) > 0 {
		preEnd = starts[0].Line - 1
	}
	pre := joinLines(lines, first, preEnd)
	if pre != "" || len(starts) == 0 {
		out = append(out, Section{Content: pre, Line: first + 1})
	}
	for i, h := range starts {
		end := len(lines)
		if i+1 < len(starts) {
			end = starts[i+1].Line - 1
		}
		out = append(out, Section{
			Heading: strings.TrimSpace(h.Text),
			Content: joinLines(lines, h.Line, end), // the heading line itself is in Heading
			Line:    h.Line,
		})
	}
	return out
}

// joinLines returns lines[from:to] joined and trimmed, clamped to range.
func joinLines(lines []string, from, to int) string {
	from, to = max(from, 0), min(to, len(lines))
	if from >= to {
		return ""
	}
	return strings.TrimSpace(strings.Join(lines[from:to], ""))
}
