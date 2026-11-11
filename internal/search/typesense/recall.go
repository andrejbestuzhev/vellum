package typesense

import (
	"context"
	"fmt"
	"time"

	"github.com/typesense/typesense-go/v3/typesense/api"
)

// RecallQuery is one recall: a hybrid search with optional filters.
type RecallQuery struct {
	Query        string
	Project      string
	Tags         []string // all must be present
	Since        time.Time
	BudgetTokens int // cap on the returned text, about 4 characters a token
	Limit        int // cap on returned notes
}

// Hit is the best section of one matching note.
type Hit struct {
	NoteID    string   `json:"note_id"`
	Title     string   `json:"title"`
	Section   string   `json:"section"`
	Content   string   `json:"content"`
	Project   string   `json:"project,omitempty"`
	Tags      []string `json:"tags,omitempty"`
	UpdatedAt string   `json:"updated_at"` // RFC 3339, UTC
	Score     float64  `json:"score"`      // rank fusion score, 0 to 1
}

// sectionsPerNote is how many sections are fetched for each note asked
// for: grouping keeps one per note, so a few long notes must not use up
// the page.
const sectionsPerNote = 5

// Recall runs the hybrid search: keyword match over title, heading and text
// fused with a vector search over the embedding, filtered, one hit per
// note (its best section), cut to the token budget.
func (s *Store) Recall(ctx context.Context, q RecallQuery) ([]Hit, error) {
	perPage := min(max(q.Limit, 1)*sectionsPerNote, 250)
	params := &api.SearchCollectionParams{
		Q:             &q.Query,
		QueryBy:       ptr("title,section,content,embedding"),
		VectorQuery:   ptr(fmt.Sprintf("embedding:([], k:%d)", perPage)),
		ExcludeFields: ptr("embedding"),
		PerPage:       &perPage,
	}
	if f := recallFilter(q.Project, q.Tags, q.Since); f != "" {
		params.FilterBy = &f
	}
	raw, err := s.search(ctx, params)
	if err != nil {
		return nil, err
	}
	hits := make([]Hit, len(raw))
	for i, h := range raw {
		hits[i] = hitOf(h.Document, h.Hybrid.Score)
	}
	hits = GroupBest(hits)
	if len(hits) > q.Limit && q.Limit > 0 {
		hits = hits[:q.Limit]
	}
	return TrimBudget(hits, q.BudgetTokens), nil
}

func hitOf(d Doc, score float64) Hit {
	return Hit{
		NoteID: d.NoteID, Title: d.Title, Section: d.Section, Content: d.Content,
		Project: d.Project, Tags: d.Tags, Score: score,
		UpdatedAt: time.Unix(d.UpdatedAt, 0).UTC().Format(time.RFC3339),
	}
}
