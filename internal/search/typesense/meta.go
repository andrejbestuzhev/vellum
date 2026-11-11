package typesense

import (
	"fmt"
	"strings"
	"time"
)

// defaultSource is the source of a note whose frontmatter names none: one
// that was put into the vault by hand.
const defaultSource = "manual"

// fmString returns a scalar frontmatter value as a trimmed string, "" when
// it is absent or a list or map.
func fmString(fm map[string]any, key string) string {
	switch v := fm[key].(type) {
	case nil, []any, map[string]any:
		return ""
	case string:
		return strings.TrimSpace(v)
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

// fmTime reads a timestamp from the frontmatter. YAML decodes an unquoted
// ISO timestamp into time.Time and a quoted one into a string; both work,
// as does a bare date.
func fmTime(fm map[string]any, key string) (time.Time, bool) {
	switch v := fm[key].(type) {
	case time.Time:
		return v, true
	case string:
		return ParseTime(v)
	}
	return time.Time{}, false
}

// ParseTime accepts RFC 3339 (with or without fractional seconds) and a
// bare date (UTC midnight).
func ParseTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// noteTimes returns a note's created and updated times as unix seconds.
// created is the frontmatter created_at, else the file time. updated is the
// later of the frontmatter updated_at and the file time, so an edit that
// leaves the frontmatter alone (by hand, or a vellum tool such as
// append_to_note) still counts as a change.
func noteTimes(fm map[string]any, mod time.Time) (created, updated int64) {
	updated = mod.Unix()
	if t, ok := fmTime(fm, "updated_at"); ok && t.Unix() > updated {
		updated = t.Unix()
	}
	created = mod.Unix()
	if t, ok := fmTime(fm, "created_at"); ok {
		created = t.Unix()
	}
	return created, max(updated, created)
}
