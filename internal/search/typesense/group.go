package typesense

import (
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// charsPerToken is the budget estimate: one token is about 4 characters.
const charsPerToken = 4

// minTail is the smallest cut of a section worth returning when the budget
// runs out in the middle of it.
const minTail = 80

// GroupBest keeps the first hit of every note, in order. Hits come ranked,
// so that is the note's best section. Typesense's own group_by cannot be
// used here: it drops hits that only the vector search found.
func GroupBest(hits []Hit) []Hit {
	seen := make(map[string]bool, len(hits))
	out := hits[:0:0]
	for _, h := range hits {
		if !seen[h.NoteID] {
			seen[h.NoteID] = true
			out = append(out, h)
		}
	}
	return out
}

// TrimBudget keeps hits while their text fits the token budget. The hit
// that overflows it is cut short (marked with "…") when enough budget is
// left to be useful, and nothing after it is returned. The first hit is
// always kept, cut if need be. budget <= 0 means no cap.
func TrimBudget(hits []Hit, budget int) []Hit {
	if budget <= 0 {
		return hits
	}
	left := budget * charsPerToken
	var out []Hit
	for _, h := range hits {
		cost := utf8.RuneCountInString(h.Title) + utf8.RuneCountInString(h.Section)
		body := utf8.RuneCountInString(h.Content)
		if cost+body <= left {
			out = append(out, h)
			left -= cost + body
			continue
		}
		if room := left - cost; room >= minTail || len(out) == 0 {
			h.Content = cutRunes(h.Content, max(room, minTail)) + "…"
			out = append(out, h)
		}
		break
	}
	return out
}

// cutRunes returns at most n runes of s.
func cutRunes(s string, n int) string {
	i := 0
	for pos := range s {
		if i == n {
			return s[:pos]
		}
		i++
	}
	return s
}

// recallFilter builds filter_by from recall's filters: tags AND-ed, as
// vellum's own tag filter does.
func recallFilter(project string, tags []string, since time.Time) string {
	var parts []string
	if project = strings.TrimSpace(project); project != "" {
		parts = append(parts, "project:="+quote(project))
	}
	for _, t := range tags {
		if t = strings.TrimSpace(strings.TrimPrefix(t, "#")); t != "" {
			parts = append(parts, "tags:="+quote(t))
		}
	}
	if !since.IsZero() {
		parts = append(parts, "updated_at:>="+strconv.FormatInt(since.Unix(), 10))
	}
	return strings.Join(parts, " && ")
}
