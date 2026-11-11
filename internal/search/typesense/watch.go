package typesense

import (
	"context"
	"sync"
	"time"

	"github.com/freema/vellum/internal/vault"
)

// Watcher timing. vellum's index notices changes made outside it (an
// editor, a copied file) only when Reconcile runs, so the watcher runs it
// on every tick; a changed path is indexed once it has been quiet for
// debounce, and retried after retryAfter when Typesense fails.
const (
	tick       = 500 * time.Millisecond
	debounce   = 2 * time.Second
	retryAfter = 30 * time.Second
)

// pending collects changed paths between flushes. due is when each path
// may be indexed next.
type pending struct {
	mu  sync.Mutex
	due map[string]time.Time
}

func (p *pending) add(path string, at time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.due[path] = at
}

// take removes and returns the paths that are due at now.
func (p *pending) take(now time.Time) []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	var out []string
	for path, at := range p.due {
		if !now.Before(at) {
			out = append(out, path)
			delete(p.due, path)
		}
	}
	return out
}

// Watch keeps the collection in step with the vault until ctx ends: it
// subscribes to the index at once and works in a goroutine of its own. Every
// change the index reports (writes through vellum, and through Reconcile
// changes on disk) is indexed after the debounce; a deleted or moved note
// is removed. IndexNote skips a note whose content is already indexed, so
// a note remember has just indexed costs no second request.
func (s *Store) Watch(ctx context.Context, ix *vault.Index) {
	p := &pending{due: map[string]time.Time{}}
	ix.OnChange(func(c vault.Change) { p.add(c.Path, time.Now().Add(debounce)) })
	go s.watch(ctx, ix, p)
}

func (s *Store) watch(ctx context.Context, ix *vault.Index, p *pending) {
	t := time.NewTicker(tick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		if _, err := ix.Reconcile(); err != nil {
			s.log.Warn("vault rescan failed", "error", err)
		}
		for _, path := range p.take(time.Now()) {
			if err := s.IndexNote(ctx, path); err != nil {
				s.log.Warn("typesense indexing failed, will retry", "path", path, "error", err)
				p.add(path, time.Now().Add(retryAfter))
			}
		}
	}
}
