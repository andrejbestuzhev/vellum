package typesense

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/typesense/typesense-go/v3/typesense/api"

	"github.com/freema/vellum/internal/vault"
)

// IndexNote reads a note and upserts its sections, then deletes the
// sections it no longer has. A note whose content was already indexed is
// skipped; a note that is gone is removed.
func (s *Store) IndexNote(ctx context.Context, path string) error {
	n, err := s.vault.Read(path)
	if errors.Is(err, vault.ErrNotFound) {
		return s.RemoveNote(ctx, path)
	}
	if err != nil {
		return err
	}
	if s.isIndexed(path, n.Hash) {
		return nil
	}
	return s.indexNote(ctx, n)
}

// indexNote upserts a note that has been read, whether or not it changed.
func (s *Store) indexNote(ctx context.Context, n *vault.Note) error {
	docs := BuildDocs(n, s.inbox)
	items := make([]any, len(docs))
	ids := make([]string, len(docs))
	for i, d := range docs {
		items[i], ids[i] = d, quote(d.ID)
	}
	cctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	action := api.Upsert
	res, err := s.docs().Import(cctx, items, &api.ImportDocumentsParams{Action: &action})
	if err != nil {
		return fmt.Errorf("index %s: %w", n.Path, err)
	}
	for _, r := range res {
		if !r.Success {
			return fmt.Errorf("index %s: %s", n.Path, r.Error)
		}
	}
	stale := fmt.Sprintf("note_id:=%s && id:!=[%s]", quote(docs[0].NoteID), strings.Join(ids, ","))
	if err := s.deleteWhere(cctx, stale); err != nil {
		return fmt.Errorf("index %s: drop old sections: %w", n.Path, err)
	}
	s.markIndexed(n.Path, n.Hash)
	return nil
}

// RemoveNote deletes every section of the note at path.
func (s *Store) RemoveNote(ctx context.Context, path string) error {
	cctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	if err := s.deleteWhere(cctx, "note_id:="+quote(NoteID(path, s.inbox))); err != nil {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	s.markIndexed(path, "")
	return nil
}

func (s *Store) deleteWhere(ctx context.Context, filter string) error {
	_, err := s.docs().Delete(ctx, &api.DeleteDocumentsParams{FilterBy: &filter})
	return err
}

// quote makes a value safe inside filter_by: backticks enclose it, so
// spaces, slashes, commas and operators in it are literal. A backtick in
// the value itself cannot be escaped and is dropped.
func quote(v string) string {
	return "`" + strings.ReplaceAll(v, "`", "") + "`"
}
