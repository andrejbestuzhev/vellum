package typesense

import (
	"path"
	"strings"
)

// NoteID is the id of the note at a vault path. A note directly in the
// inbox is known by its file name without .md (the ULID remember gives
// it); any other note by its whole vault path, so ids never collide and
// NotePath can turn one back into its path. A bare id has neither a slash
// nor a dot; an inbox file whose name has more dots keeps its path too.
func NoteID(notePath, inbox string) string {
	dir, name := path.Split(notePath)
	stem, ok := strings.CutSuffix(name, ".md")
	if ok && stem != "" && !strings.Contains(stem, ".") && path.Clean(dir) == path.Clean(inbox) {
		return stem
	}
	return notePath
}

// NotePath is the vault path of a note id (the inverse of NoteID).
func NotePath(id, inbox string) string {
	if strings.ContainsAny(id, "/.") {
		return id
	}
	return path.Join(inbox, id+".md")
}
