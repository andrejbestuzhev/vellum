package mcpserver

import (
	"errors"
	"fmt"

	"github.com/freema/vellum/internal/search/typesense"
)

// Defaults and caps of recall.
const (
	recallBudget   = 1500
	recallLimit    = 20
	recallMaxLimit = 50
)

type recallIn struct {
	Query        string   `json:"query" jsonschema:"what to recall, in any words and language; matched by meaning and by keywords"`
	Project      string   `json:"project,omitempty" jsonschema:"only notes of this project"`
	Tags         []string `json:"tags,omitempty" jsonschema:"only notes carrying all of these tags"`
	Since        string   `json:"since,omitempty" jsonschema:"only notes changed since then: ISO 8601 date or date-time, e.g. 2026-11-01 or 2026-11-01T18:00:00Z"`
	BudgetTokens int      `json:"budget_tokens,omitempty" jsonschema:"cap on the returned text in tokens (default 1500, about 4 characters a token); the last result is cut to fit"`
	Limit        int      `json:"limit,omitempty" jsonschema:"cap on returned notes (default 20, at most 50)"`
}
type recallOut struct {
	Results []typesense.Hit `json:"results"`
}

type digestIn struct {
	Since   string `json:"since" jsonschema:"ISO 8601 date or date-time: notes created or changed since then"`
	Project string `json:"project,omitempty" jsonschema:"only notes of this project"`
}

// query validates recall's input and applies the defaults.
func (in recallIn) query() (typesense.RecallQuery, error) {
	q := typesense.RecallQuery{Query: in.Query, Project: in.Project, Tags: in.Tags,
		BudgetTokens: in.BudgetTokens, Limit: in.Limit}
	if q.Query == "" {
		return q, errors.New("query is required")
	}
	if in.Since != "" {
		t, ok := typesense.ParseTime(in.Since)
		if !ok {
			return q, fmt.Errorf("since must be an ISO 8601 date or date-time, got %q", in.Since)
		}
		q.Since = t
	}
	if q.BudgetTokens <= 0 {
		q.BudgetTokens = recallBudget
	}
	q.Limit = min(max(q.Limit, 0), recallMaxLimit)
	if q.Limit == 0 {
		q.Limit = recallLimit
	}
	return q, nil
}
