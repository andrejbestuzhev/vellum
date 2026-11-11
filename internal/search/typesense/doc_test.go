package typesense

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/freema/vellum/internal/vault"
)

func readNote(t *testing.T, rel, content string) *vault.Note {
	t.Helper()
	dir := t.TempDir()
	abs := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	v, err := vault.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	n, err := v.Read(rel)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestBuildDocs(t *testing.T) {
	n := readNote(t, "inbox/01JC.md", `---
id: 01JC
title: Deploy notes
created_at: 2026-11-11T18:00:00Z
updated_at: "2026-11-11T19:00:00Z"
source: claude-code
project: infra
tags: [deploy, traefik]
attachments: [a.png]
---
Why we deploy this way.

## Steps
Run compose.
`)
	created := time.Date(2026, 11, 11, 18, 0, 0, 0, time.UTC).Unix()
	updated := max(time.Date(2026, 11, 11, 19, 0, 0, 0, time.UTC).Unix(), n.ModTime.Unix())
	base := Doc{NoteID: "01JC", Title: "Deploy notes", Project: "infra", Tags: []string{"deploy", "traefik"},
		Source: "claude-code", CreatedAt: created, UpdatedAt: updated}
	want := []Doc{base, base}
	want[0].ID, want[0].Content = "01JC#0", "Why we deploy this way."
	want[1].ID, want[1].Section, want[1].Content = "01JC#1", "Steps", "Run compose."
	if got := BuildDocs(n, "inbox"); !reflect.DeepEqual(got, want) {
		t.Errorf("BuildDocs:\n got %+v\nwant %+v", got, want)
	}
}

func TestBuildDocsWithoutFrontmatter(t *testing.T) {
	n := readNote(t, "inbox/hand.md", "# Typed by hand\nSome text.\n")
	docs := BuildDocs(n, "inbox")
	if len(docs) != 1 {
		t.Fatalf("got %d docs, want 1", len(docs))
	}
	d := docs[0]
	mod := n.ModTime.Unix()
	if d.NoteID != "hand" || d.Title != "Typed by hand" || d.Source != defaultSource ||
		d.CreatedAt != mod || d.UpdatedAt != mod || d.Project != "" {
		t.Errorf("unexpected doc %+v", d)
	}
}

func TestNoteTimesPrefersLaterFileTime(t *testing.T) {
	fm := map[string]any{"created_at": time.Unix(100, 0), "updated_at": time.Unix(200, 0)}
	if c, u := noteTimes(fm, time.Unix(500, 0)); c != 100 || u != 500 {
		t.Errorf("edited after updated_at: got %d, %d", c, u)
	}
	if c, u := noteTimes(fm, time.Unix(150, 0)); c != 100 || u != 200 {
		t.Errorf("frontmatter newer: got %d, %d", c, u)
	}
}
