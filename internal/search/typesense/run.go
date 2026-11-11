package typesense

import (
	"context"
	"time"

	"github.com/freema/vellum/internal/vault"
)

// ensureRetry is how long Run waits between attempts to reach Typesense.
const ensureRetry = 10 * time.Second

// Run is the server side of the index, meant for a goroutine: it waits for
// Typesense and the collection, catches up with changes made while the
// server was down, then watches the vault until ctx ends. The server
// answers meanwhile; recall fails until the collection exists.
func (s *Store) Run(ctx context.Context, ix *vault.Index) {
	for {
		created, err := s.Ensure(ctx)
		if err == nil {
			if created {
				s.log.Info("typesense collection created", "collection", s.cfg.Collection)
			}
			break
		}
		s.log.Warn("typesense not ready, retrying", "url", s.cfg.URL, "error", err, "in", ensureRetry.String())
		select {
		case <-ctx.Done():
			return
		case <-time.After(ensureRetry):
		}
	}
	// Watch subscribes before the catch-up, so nothing changed during it is lost.
	s.Watch(ctx, ix)
	st, err := s.Sync(ctx, ix, false)
	if err != nil {
		s.log.Warn("typesense catch-up failed; the nightly reindex will repair it", "error", err)
		return
	}
	s.log.Info("typesense index in step with the vault", "collection", s.cfg.Collection,
		"notes", st.Files, "indexed", st.Indexed, "removed", st.Removed, "failed", st.Failed)
}

// Reindex rebuilds the collection from the vault: every note is indexed
// again and notes gone from the vault are removed. wipe drops the
// collection first. The counts of notes and of distinct note ids in the
// collection are compared at the end; a difference is logged as a warning.
func (s *Store) Reindex(ctx context.Context, ix *vault.Index, wipe bool) (SyncStats, error) {
	if wipe {
		if err := s.Drop(ctx); err != nil {
			return SyncStats{}, err
		}
	}
	if _, err := s.Ensure(ctx); err != nil {
		return SyncStats{}, err
	}
	st, err := s.Sync(ctx, ix, true)
	if err != nil {
		return st, err
	}
	if st.InIndex != st.Files {
		s.log.Warn("reindex: counts differ", "files", st.Files, "note_ids", st.InIndex, "failed", st.Failed)
	}
	return st, nil
}
