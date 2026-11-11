package mcpserver

import (
	"context"
	"errors"
	"log/slog"
	"path"
	"strings"
	"time"
	"unicode/utf8"

	"gopkg.in/yaml.v3"

	"github.com/freema/vellum/internal/search/typesense"
	"github.com/freema/vellum/internal/vault"
)

type rememberIn struct {
	Text    string   `json:"text" jsonschema:"the note itself, markdown; ## headings split a long note into searchable sections"`
	Title   string   `json:"title,omitempty" jsonschema:"note title; default: the first line of text, up to 80 characters"`
	Project string   `json:"project,omitempty" jsonschema:"project the note belongs to"`
	Tags    []string `json:"tags,omitempty" jsonschema:"free-form tags"`
	Source  string   `json:"source,omitempty" jsonschema:"where the note comes from: chat (default), claude-code, manual or hook"`
}
type rememberOut struct {
	ID      string `json:"id"`
	Path    string `json:"path"`
	Indexed bool   `json:"indexed"` // false: saved, searchable once the index catches up
}

// noteMeta is the frontmatter remember writes, in this order.
type noteMeta struct {
	ID        string    `yaml:"id"`
	Title     string    `yaml:"title"`
	CreatedAt time.Time `yaml:"created_at"`
	UpdatedAt time.Time `yaml:"updated_at"`
	Source    string    `yaml:"source"`
	Project   string    `yaml:"project,omitempty"`
	Tags      []string  `yaml:"tags,omitempty,flow"`
}

// maxTitle is the length cap of a title taken from the text.
const maxTitle = 80

// remember writes inbox/<ulid>.md and indexes it right away. A failed
// indexing does not undo the write: the note is saved, the answer says
// indexed: false, and the watcher retries.
func remember(ctx context.Context, d Deps, store *typesense.Store, in rememberIn) (rememberOut, error) {
	text := strings.TrimSpace(in.Text)
	if text == "" {
		return rememberOut{}, errors.New("text is required")
	}
	now := time.Now().UTC().Truncate(time.Second)
	meta := noteMeta{ID: newULID(now), Title: strings.TrimSpace(in.Title), CreatedAt: now, UpdatedAt: now,
		Source: strings.TrimSpace(in.Source), Project: strings.TrimSpace(in.Project), Tags: in.Tags}
	if meta.Title == "" {
		meta.Title = titleOf(text)
	}
	if meta.Source == "" {
		meta.Source = "chat"
	}
	fm, err := yaml.Marshal(meta)
	if err != nil {
		return rememberOut{}, err
	}
	p := path.Join(d.Structure.Inbox, meta.ID+".md")
	if err := d.Vault.Write(p, "---\n"+string(fm)+"---\n"+text+"\n", vault.WriteOptions{}); err != nil {
		return rememberOut{}, err
	}
	_ = d.Index.Update(p)
	out := rememberOut{ID: meta.ID, Path: p, Indexed: true}
	if err := store.IndexNote(ctx, p); err != nil {
		slog.Warn("remember: note saved but not indexed", "path", p, "error", err)
		out.Indexed = false
	}
	return out, nil
}

// titleOf is the first non-blank line of text without heading marks, cut
// to maxTitle characters on a word boundary when there is one.
func titleOf(text string) string {
	line, _, _ := strings.Cut(text, "\n")
	line = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "#"))
	if utf8.RuneCountInString(line) <= maxTitle {
		return line
	}
	cut := string([]rune(line)[:maxTitle])
	if i := strings.LastIndex(cut, " "); i > maxTitle/2 {
		cut = cut[:i]
	}
	return strings.TrimSpace(cut)
}
