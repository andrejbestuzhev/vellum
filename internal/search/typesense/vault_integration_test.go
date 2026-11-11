//go:build integration

package typesense_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type hit struct {
	NoteID  string `json:"note_id"`
	Section string `json:"section"`
}

// recallFinds polls recall until the note is the top hit or wait runs out,
// and returns how long it took.
func (r *rig) recallFinds(t *testing.T, query, noteID string, wait time.Duration) time.Duration {
	t.Helper()
	start := time.Now()
	for {
		var out struct{ Results []hit }
		r.call(t, "recall", map[string]any{"query": query}, &out)
		if len(out.Results) > 0 && out.Results[0].NoteID == noteID {
			return time.Since(start)
		}
		if time.Since(start) > wait {
			t.Fatalf("recall %q: %s not on top after %s: %+v", query, noteID, wait, out.Results)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func TestRememberThenRecallByMeaning(t *testing.T) {
	r := newRig(t)
	var saved struct {
		ID      string
		Path    string
		Indexed bool
	}
	r.call(t, "remember", map[string]any{
		"text":    "Postgres backups\n\npg_dump runs every night from cron and the archive is copied off the server.\n\n## Restore\nTested on staging: pg_restore into an empty database.",
		"project": "infra", "tags": []string{"postgres"},
	}, &saved)
	if !saved.Indexed || saved.Path != "inbox/"+saved.ID+".md" {
		t.Fatalf("remember: %+v", saved)
	}
	r.call(t, "remember", map[string]any{"text": "Borscht recipe: beetroot, cabbage, potatoes; simmer for ninety minutes."}, &struct{}{})
	// Other words, no shared keyword: found by meaning.
	t.Logf("remember -> recall: %s", r.recallFinds(t, "how to recover lost records", saved.ID, 3*time.Second))
	var tools struct{ Results []hit }
	r.call(t, "recall", map[string]any{"query": "database restore", "project": "garden"}, &tools)
	if len(tools.Results) != 0 {
		t.Errorf("project filter ignored: %+v", tools.Results)
	}
	list, err := r.session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range list.Tools {
		if tool.Name == "search_notes" {
			t.Error("search_notes must not be registered with the vault tools")
		}
	}
	var dg struct {
		Created []hit
		Stats   struct {
			BySource map[string]int `json:"by_source"`
		}
	}
	r.call(t, "digest", map[string]any{"since": time.Now().Add(-24 * time.Hour).Format(time.RFC3339)}, &dg)
	if len(dg.Created) != 2 || dg.Stats.BySource["chat"] != 2 {
		t.Errorf("digest: %+v", dg)
	}
}

func TestHandWrittenFileReachesRecall(t *testing.T) {
	r := newRig(t)
	abs := filepath.Join(r.vault.Root(), "inbox", "handmade.md")
	_ = os.MkdirAll(filepath.Dir(abs), 0o755)
	if err := os.WriteFile(abs, []byte("# Mountain trip\n\nTent, sleeping bags, a gas stove and a first aid kit.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("file on disk -> recall: %s", r.recallFinds(t, "what to pack for a hike", "handmade", 5*time.Second))
	if err := os.Remove(abs); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		var out struct{ Results []hit }
		r.call(t, "recall", map[string]any{"query": "camping gear"}, &out)
		if len(out.Results) == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("deleted note still found: %+v", out.Results)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func TestReindexWipeRestoresCount(t *testing.T) {
	r := newRig(t)
	for _, text := range []string{"First note\n\n## A\nalpha\n## B\nbeta", "Second note", "Third note"} {
		r.call(t, "remember", map[string]any{"text": text}, &struct{}{})
	}
	st, err := r.store.Reindex(context.Background(), r.index, true)
	if err != nil {
		t.Fatal(err)
	}
	if st.Files != 3 || st.InIndex != 3 || st.Indexed != 3 || st.Failed != 0 {
		t.Errorf("reindex --wipe: %+v", st)
	}
}
