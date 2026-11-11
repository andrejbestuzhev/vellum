package typesense

import (
	"reflect"
	"testing"
)

func TestSplitSections(t *testing.T) {
	fm := "---\ntitle: T\n---\n"
	cases := []struct {
		name    string
		content string
		body    string
		want    []Section
	}{
		{
			name:    "preamble and sections",
			content: fm + "Intro line.\n\n## First\nOne.\n### Deeper\nStill one.\n## Second\nTwo.\n",
			body:    "Intro line.\n\n## First\nOne.\n### Deeper\nStill one.\n## Second\nTwo.\n",
			want: []Section{
				{Heading: "", Content: "Intro line.", Line: 4},
				{Heading: "First", Content: "One.\n### Deeper\nStill one.", Line: 6},
				{Heading: "Second", Content: "Two.", Line: 10},
			},
		},
		{
			name:    "blank preamble is left out",
			content: "\n## Only\nText\n",
			body:    "\n## Only\nText\n",
			want:    []Section{{Heading: "Only", Content: "Text", Line: 2}},
		},
		{
			name:    "no sections keeps one even if empty",
			content: fm,
			body:    "",
			want:    []Section{{Heading: "", Content: "", Line: 4}},
		},
		{
			name:    "heading in fenced code is not a section",
			content: "Code:\n```\n## not a heading\n```\n",
			body:    "Code:\n```\n## not a heading\n```\n",
			want:    []Section{{Heading: "", Content: "Code:\n```\n## not a heading\n```", Line: 1}},
		},
		{
			name:    "level one heading stays in the preamble",
			content: "# Title\nText\n## Part\nMore\n",
			body:    "# Title\nText\n## Part\nMore\n",
			want: []Section{
				{Heading: "", Content: "# Title\nText", Line: 1},
				{Heading: "Part", Content: "More", Line: 3},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := SplitSections(c.content, c.body); !reflect.DeepEqual(got, c.want) {
				t.Errorf("SplitSections:\n got %#v\nwant %#v", got, c.want)
			}
		})
	}
}

func TestNoteIDRoundTrip(t *testing.T) {
	cases := map[string]string{
		"inbox/01JC0000000000000000000000.md": "01JC0000000000000000000000",
		"inbox/my.note.md":                    "inbox/my.note.md",
		"inbox/sub/x.md":                      "inbox/sub/x.md",
		"projects/a b.md":                     "projects/a b.md",
		"root.md":                             "root.md",
		"inbox/x.markdown":                    "inbox/x.markdown",
	}
	for path, id := range cases {
		if got := NoteID(path, "inbox"); got != id {
			t.Errorf("NoteID(%q) = %q, want %q", path, got, id)
		}
		if back := NotePath(id, "inbox"); back != path {
			t.Errorf("NotePath(%q) = %q, want %q", id, back, path)
		}
	}
}
