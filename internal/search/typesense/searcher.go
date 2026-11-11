package typesense

import (
	"context"
	"strings"

	"github.com/freema/vellum/internal/vault"
)

// maxSearcherNotes caps one Searcher call; vault.SearchPage asks a plain
// Searcher for everything and cuts the page itself.
const maxSearcherNotes = 100

// Searcher is vellum's search interface over the collection: the hybrid
// search, one result per note with its best section as the snippet. What
// the collection cannot answer goes to fallback (vellum's scan): regex
// queries, an empty query (a pure tag filter), a folder scope, and any
// query while Typesense fails, so the web search keeps working.
type Searcher struct {
	store    *Store
	fallback vault.Searcher
}

// Searcher returns the vault.Searcher of the store.
func (s *Store) Searcher(fallback vault.Searcher) *Searcher {
	return &Searcher{store: s, fallback: fallback}
}

// Search implements vault.Searcher.
func (x *Searcher) Search(query string, opts vault.SearchOpts) ([]vault.Result, error) {
	if opts.Regex || strings.TrimSpace(query) == "" || opts.Dir != "" {
		return x.fallback.Search(query, opts)
	}
	limit := opts.Offset + opts.MaxResults
	if opts.MaxResults <= 0 || limit > maxSearcherNotes {
		limit = maxSearcherNotes
	}
	hits, err := x.store.Recall(context.Background(), RecallQuery{Query: query, Tags: opts.Tags, Limit: limit})
	if err != nil {
		x.store.log.Warn("typesense search failed, using the scan", "error", err)
		return x.fallback.Search(query, opts)
	}
	out := make([]vault.Result, 0, len(hits))
	for _, h := range hits {
		if h.Score < opts.MinScore {
			continue
		}
		path := NotePath(h.NoteID, x.store.inbox)
		out = append(out, vault.Result{
			Path:  path,
			Title: h.Title,
			Score: h.Score,
			Snippets: []vault.Snippet{{
				Line:    x.sectionLine(path, h.Section),
				Context: cutRunes(h.Content, 300),
			}},
		})
	}
	if opts.Offset > 0 {
		out = out[min(opts.Offset, len(out)):]
	}
	return out, nil
}

// sectionLine finds the file line of a hit's section heading, so the web
// UI can jump to it; 1 when the note changed meanwhile.
func (x *Searcher) sectionLine(path, heading string) int {
	n, err := x.store.vault.Read(path)
	if err != nil {
		return 1
	}
	for _, sec := range SplitSections(n.Content, n.Body) {
		if sec.Heading == heading {
			return sec.Line
		}
	}
	return 1
}
