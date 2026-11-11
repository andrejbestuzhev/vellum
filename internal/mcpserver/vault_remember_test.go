package mcpserver

import (
	"strings"
	"testing"
	"time"
)

func TestNewULID(t *testing.T) {
	t0 := time.UnixMilli(1_700_000_000_000)
	a, b := newULID(t0), newULID(t0.Add(time.Millisecond))
	if len(a) != 26 || strings.Trim(a, crockford) != "" {
		t.Fatalf("not a ULID: %q", a)
	}
	if a[:10] >= b[:10] {
		t.Errorf("time prefix must sort: %q then %q", a, b)
	}
	if a[0] > '7' {
		t.Errorf("first character carries 3 bits, got %q", a)
	}
	if newULID(t0) == a {
		t.Error("random part repeats")
	}
}

func TestTitleOf(t *testing.T) {
	long := strings.Repeat("café ", 30)
	cases := map[string]string{
		"## Deploy notes\nbody":  "Deploy notes",
		"Short line":             "Short line",
		long:                     strings.TrimSpace(strings.Repeat("café ", 16)),
		strings.Repeat("x", 100): strings.Repeat("x", 80),
	}
	for in, want := range cases {
		if got := titleOf(in); got != want {
			t.Errorf("titleOf(%.20q) = %q, want %q", in, got, want)
		}
	}
}

func TestRecallDefaults(t *testing.T) {
	q, err := recallIn{Query: "x", Limit: 500, Since: "2026-11-01"}.query()
	if err != nil {
		t.Fatal(err)
	}
	if q.BudgetTokens != recallBudget || q.Limit != recallMaxLimit || q.Since.IsZero() {
		t.Errorf("defaults not applied: %+v", q)
	}
	if _, err := (recallIn{}).query(); err == nil {
		t.Error("empty query must fail")
	}
	if _, err := (recallIn{Query: "x", Since: "yesterday"}).query(); err == nil {
		t.Error("bad since must fail")
	}
}
