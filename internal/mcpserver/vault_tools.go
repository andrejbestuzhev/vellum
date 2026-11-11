package mcpserver

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/freema/vellum/internal/search/typesense"
)

// VaultInstructions is appended to the server instructions when the vault
// tools are on (Deps.ExtraInstructions).
const VaultInstructions = `

Memory tools (this server): recall is the only search tool here — search_notes is not available. For context call recall, not read_note or list_notes: it returns the best matching section of each note, ranked by meaning and by words, within a token budget. When asked to note, save, fix or remember something, call remember. All notes live in one inbox folder; the only structure is project and tags — do not create folders or note types. digest lists what was created or changed since a date.`

// VaultTools returns the Deps.Extend hook of the vault build: remember,
// recall and digest over the Typesense store. search_notes is removed, so
// recall is the one way to search.
func VaultTools(store *typesense.Store) func(*mcp.Server, Deps) {
	return func(s *mcp.Server, d Deps) {
		s.RemoveTools("search_notes")

		mcp.AddTool(s, editTool("remember", "Remember",
			"Save a new note to the inbox: text is the markdown body. Without a title the first line (up to 80 characters) is used. Returns the note id, its path and whether it is already searchable (indexed).",
			false, false, // always creates a new note
		), func(ctx context.Context, req *mcp.CallToolRequest, in rememberIn) (*mcp.CallToolResult, rememberOut, error) {
			out, err := remember(ctx, d, store, in)
			return nil, out, err
		})

		mcp.AddTool(s, readTool("recall", "Recall",
			"Hybrid search of the notes (meaning and keywords together), the only search tool. Returns one result per note — its best matching section with title, project, tags, updated_at and a score — cut to budget_tokens. Filters: project, tags (all), since.",
		), func(ctx context.Context, req *mcp.CallToolRequest, in recallIn) (*mcp.CallToolResult, recallOut, error) {
			q, err := in.query()
			if err != nil {
				return nil, recallOut{}, err
			}
			hits, err := store.Recall(ctx, q)
			if err != nil {
				return nil, recallOut{}, err
			}
			if hits == nil {
				hits = []typesense.Hit{}
			}
			return nil, recallOut{Results: hits}, nil
		})

		mcp.AddTool(s, readTool("digest", "Digest",
			"Notes created or changed since a date, newest first, optionally of one project: created and updated lists plus counts by project and by source. A plain list, no ranking; write the summary yourself.",
		), func(ctx context.Context, req *mcp.CallToolRequest, in digestIn) (*mcp.CallToolResult, typesense.Digest, error) {
			since, ok := typesense.ParseTime(in.Since)
			if !ok {
				return nil, typesense.Digest{}, fmt.Errorf("since must be an ISO 8601 date or date-time, got %q", in.Since)
			}
			out, err := store.Digest(ctx, since, in.Project)
			return nil, out, err
		})
	}
}
