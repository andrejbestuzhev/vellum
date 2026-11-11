package typesense

import (
	"context"
	"time"

	"github.com/typesense/typesense-go/v3/typesense/api"
)

// DigestNote is one note in a digest.
type DigestNote struct {
	NoteID    string   `json:"note_id"`
	Title     string   `json:"title"`
	Project   string   `json:"project,omitempty"`
	Tags      []string `json:"tags,omitempty"`
	Source    string   `json:"source"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
}

// Digest lists the notes created or changed since a time, newest first.
// A note created in the window is in Created only.
type Digest struct {
	Created []DigestNote `json:"created"`
	Updated []DigestNote `json:"updated"`
	Stats   DigestStats  `json:"stats"`
}

// DigestStats counts the digest's notes (created and updated together).
type DigestStats struct {
	ByProject map[string]int `json:"by_project"` // notes without a project under ""
	BySource  map[string]int `json:"by_source"`
}

// digestPages caps the pages a digest reads (250 sections each).
const digestPages = 40

// Digest selects by updated_at >= since, optionally in one project. No
// ranking and no semantics: a plain list for the agent to summarize.
func (s *Store) Digest(ctx context.Context, since time.Time, project string) (Digest, error) {
	d := Digest{Created: []DigestNote{}, Updated: []DigestNote{},
		Stats: DigestStats{ByProject: map[string]int{}, BySource: map[string]int{}}}
	filter := recallFilter(project, nil, since)
	if filter == "" {
		filter = "updated_at:>=0"
	}
	seen := map[string]bool{}
	perPage := 250
	for page := 1; page <= digestPages; page++ {
		params := &api.SearchCollectionParams{
			Q: ptr("*"), FilterBy: &filter, SortBy: ptr("updated_at:desc"),
			IncludeFields: ptr("note_id,title,project,tags,source,created_at,updated_at"),
			PerPage:       &perPage, Page: &page,
		}
		hits, err := s.search(ctx, params)
		if err != nil {
			return d, err
		}
		for _, h := range hits {
			n := digestNote(h.Document)
			if seen[n.NoteID] {
				continue
			}
			seen[n.NoteID] = true
			if created, _ := ParseTime(n.CreatedAt); !created.Before(since) {
				d.Created = append(d.Created, n)
			} else {
				d.Updated = append(d.Updated, n)
			}
			d.Stats.ByProject[n.Project]++
			d.Stats.BySource[n.Source]++
		}
		if len(hits) < perPage {
			break
		}
	}
	return d, nil
}

func digestNote(doc Doc) DigestNote {
	return DigestNote{
		NoteID: doc.NoteID, Title: doc.Title, Project: doc.Project, Tags: doc.Tags, Source: doc.Source,
		CreatedAt: time.Unix(doc.CreatedAt, 0).UTC().Format(time.RFC3339),
		UpdatedAt: time.Unix(doc.UpdatedAt, 0).UTC().Format(time.RFC3339),
	}
}
