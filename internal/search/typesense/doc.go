package typesense

import (
	"strconv"

	"github.com/freema/vellum/internal/vault"
)

// Doc is one document of the collection: a section of a note.
type Doc struct {
	ID        string   `json:"id"` // <note_id>#<section index>
	NoteID    string   `json:"note_id"`
	Title     string   `json:"title"`
	Section   string   `json:"section"`
	Content   string   `json:"content"`
	Project   string   `json:"project,omitempty"`
	Tags      []string `json:"tags,omitempty"`
	Source    string   `json:"source"`
	CreatedAt int64    `json:"created_at"`
	UpdatedAt int64    `json:"updated_at"`
}

// BuildDocs turns a note into its documents, one per section, numbered
// from 0 in file order. Tags are the note's tags as vellum reads them
// (frontmatter and inline); other frontmatter fields are not indexed.
func BuildDocs(n *vault.Note, inbox string) []Doc {
	id := NoteID(n.Path, inbox)
	source := fmString(n.Frontmatter, "source")
	if source == "" {
		source = defaultSource
	}
	created, updated := noteTimes(n.Frontmatter, n.ModTime)
	sections := SplitSections(n.Content, n.Body)
	docs := make([]Doc, len(sections))
	for i, sec := range sections {
		docs[i] = Doc{
			ID:        id + "#" + strconv.Itoa(i),
			NoteID:    id,
			Title:     n.Title,
			Section:   sec.Heading,
			Content:   sec.Content,
			Project:   fmString(n.Frontmatter, "project"),
			Tags:      n.Tags,
			Source:    source,
			CreatedAt: created,
			UpdatedAt: updated,
		}
	}
	return docs
}
