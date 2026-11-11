// Package typesense keeps a Typesense collection of note sections in step
// with the vault and answers hybrid (keyword + vector) queries over it. The
// collection is a rebuildable index: the markdown files stay the source of
// truth, and every failure here leaves them untouched.
package typesense

import (
	"errors"
	"os"
	"strings"
)

// Defaults of the TYPESENSE_* settings.
const (
	DefaultCollection = "vault_sections"
	DefaultEmbedModel = "ts/multilingual-e5-small"
)

// Config is the Typesense connection, read from the environment.
type Config struct {
	URL        string // TYPESENSE_URL, e.g. http://typesense:8108
	APIKey     string // TYPESENSE_API_KEY
	Collection string // TYPESENSE_COLLECTION
	EmbedModel string // TYPESENSE_EMBED_MODEL, a built-in model name
}

// Enabled reports whether VAULT_SEARCH selects the Typesense search. Unset
// (or anything else) keeps vellum's in-memory search and no vault tools.
func Enabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("VAULT_SEARCH")), "typesense")
}

// ConfigFromEnv reads the TYPESENSE_* variables. URL and API key have no
// default.
func ConfigFromEnv() (Config, error) {
	c := Config{
		URL:        strings.TrimRight(strings.TrimSpace(os.Getenv("TYPESENSE_URL")), "/"),
		APIKey:     strings.TrimSpace(os.Getenv("TYPESENSE_API_KEY")),
		Collection: getenv("TYPESENSE_COLLECTION", DefaultCollection),
		EmbedModel: getenv("TYPESENSE_EMBED_MODEL", DefaultEmbedModel),
	}
	switch {
	case c.URL == "":
		return c, errors.New("TYPESENSE_URL is not set")
	case c.APIKey == "":
		return c, errors.New("TYPESENSE_API_KEY is not set")
	}
	return c, nil
}

func getenv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
