package typesense

import (
	"strings"
	"testing"
	"time"
)

func TestGroupBestKeepsFirstSectionPerNote(t *testing.T) {
	hits := []Hit{
		{NoteID: "a", Section: "best of a"},
		{NoteID: "b", Section: "best of b"},
		{NoteID: "a", Section: "second of a"},
		{NoteID: "c", Section: "only c"},
		{NoteID: "b", Section: "second of b"},
	}
	got := GroupBest(hits)
	want := []string{"best of a", "best of b", "only c"}
	if len(got) != len(want) {
		t.Fatalf("got %d hits, want %d", len(got), len(want))
	}
	for i, h := range got {
		if h.Section != want[i] {
			t.Errorf("hit %d = %q, want %q", i, h.Section, want[i])
		}
	}
	if hits[2].Section != "second of a" {
		t.Error("GroupBest modified its input")
	}
}

func TestTrimBudget(t *testing.T) {
	long := strings.Repeat("é", 1000)
	hits := []Hit{
		{NoteID: "a", Title: "t", Content: strings.Repeat("x", 100)},
		{NoteID: "b", Title: "t", Content: long},
		{NoteID: "c", Title: "t", Content: "never reached"},
	}
	got := TrimBudget(hits, 100) // 400 characters
	if len(got) != 2 {
		t.Fatalf("got %d hits, want 2", len(got))
	}
	if got[0].Content != hits[0].Content {
		t.Error("first hit fits and must stay whole")
	}
	cut := got[1].Content
	if !strings.HasSuffix(cut, "…") || !strings.HasPrefix(long, strings.TrimSuffix(cut, "…")) {
		t.Errorf("second hit not cut cleanly: %q", cut[:20])
	}
	if n := len([]rune(cut)) - 1; n != 400-101-1 {
		t.Errorf("cut to %d runes, want %d", n, 400-101-1)
	}
}

func TestTrimBudgetKeepsFirstHitAndStopsWhenNoRoom(t *testing.T) {
	hits := []Hit{{NoteID: "a", Content: strings.Repeat("x", 50)}, {NoteID: "b", Content: strings.Repeat("y", 500)}}
	got := TrimBudget(hits, 20) // 80 characters: the second hit gets 30, under minTail
	if len(got) != 1 {
		t.Fatalf("got %d hits, want 1", len(got))
	}
	got = TrimBudget(hits[1:], 1)
	if len(got) != 1 || len([]rune(got[0].Content)) != minTail+1 {
		t.Errorf("first hit must be kept, cut to minTail: %+v", got)
	}
	if all := TrimBudget(hits, 0); len(all) != 2 {
		t.Error("budget 0 must not cap")
	}
}

func TestRecallFilter(t *testing.T) {
	since := time.Unix(1700000000, 0)
	got := recallFilter("my proj", []string{"a", "#b", " "}, since)
	want := "project:=`my proj` && tags:=`a` && tags:=`b` && updated_at:>=1700000000"
	if got != want {
		t.Errorf("recallFilter = %q, want %q", got, want)
	}
	if got := recallFilter("", nil, time.Time{}); got != "" {
		t.Errorf("no filters = %q, want empty", got)
	}
}
