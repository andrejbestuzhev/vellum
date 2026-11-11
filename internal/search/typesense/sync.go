package typesense

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/typesense/typesense-go/v3/typesense/api"

	"github.com/freema/vellum/internal/vault"
)

// exportTimeout bounds reading the note ids of the whole collection.
const exportTimeout = 5 * time.Minute

// SyncStats says what one Sync did.
type SyncStats struct {
	Files   int // notes in the vault
	Indexed int // notes (re)indexed
	Removed int // notes dropped from the collection, gone from the vault
	Failed  int // notes that could not be indexed (logged)
	InIndex int // distinct note ids in the collection afterwards
}

// Sync brings the collection in line with the vault: notes missing from it
// or changed on disk since they were indexed are indexed, notes no longer
// in the vault are removed. force indexes every note (reindex). It is the
// startup catch-up for changes made while the server was down, and the
// reindex command.
func (s *Store) Sync(ctx context.Context, ix *vault.Index, force bool) (SyncStats, error) {
	known, err := s.noteIDs(ctx)
	if err != nil {
		return SyncStats{}, err
	}
	entries := ix.All()
	st := SyncStats{Files: len(entries)}
	inVault := make(map[string]bool, len(entries))
	for _, e := range entries {
		id := NoteID(e.Path, s.inbox)
		inVault[id] = true
		if updated, ok := known[id]; ok && !force && e.ModTime <= updated {
			continue
		}
		n, err := s.vault.Read(e.Path)
		if err == nil {
			err = s.indexNote(ctx, n)
		}
		if err != nil {
			st.Failed++
			s.log.Warn("typesense indexing failed", "path", e.Path, "error", err)
			continue
		}
		st.Indexed++
	}
	for id := range known {
		if !inVault[id] {
			if err := s.RemoveNote(ctx, NotePath(id, s.inbox)); err != nil {
				return st, err
			}
			st.Removed++
		}
	}
	after, err := s.noteIDs(ctx)
	st.InIndex = len(after)
	return st, err
}

// noteIDs reads the distinct note ids of the collection with the newest
// updated_at of each. Exported rather than grouped: a grouped search only
// estimates the number of groups.
func (s *Store) noteIDs(ctx context.Context) (map[string]int64, error) {
	cctx, cancel := context.WithTimeout(ctx, exportTimeout)
	defer cancel()
	body, err := s.docs().Export(cctx, &api.ExportDocumentsParams{IncludeFields: ptr("note_id,updated_at")})
	if err != nil {
		return nil, fmt.Errorf("typesense export: %w", err)
	}
	defer body.Close()
	ids := map[string]int64{}
	sc := bufio.NewScanner(body)
	sc.Buffer(make([]byte, 64*1024), 16*1024*1024)
	for sc.Scan() {
		var d Doc
		if err := json.Unmarshal(sc.Bytes(), &d); err != nil {
			return nil, fmt.Errorf("typesense export: %w", err)
		}
		ids[d.NoteID] = max(ids[d.NoteID], d.UpdatedAt)
	}
	return ids, sc.Err()
}
