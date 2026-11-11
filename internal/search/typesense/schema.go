package typesense

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	ts "github.com/typesense/typesense-go/v3/typesense"
	"github.com/typesense/typesense-go/v3/typesense/api"
)

// createTimeout bounds creating the collection: the first one on a
// Typesense instance downloads the embedding model (about 3 minutes for
// multilingual-e5-small, measured).
const createTimeout = 15 * time.Minute

// schemaJSON is the vault_sections collection: one document per note
// section, embedded by Typesense itself from title, heading and text.
// Name and model are filled in by schema.
const schemaJSON = `{
  "default_sorting_field": "updated_at",
  "fields": [
    {"name": "note_id", "type": "string", "facet": true},
    {"name": "title", "type": "string"},
    {"name": "section", "type": "string"},
    {"name": "content", "type": "string"},
    {"name": "embedding", "type": "float[]",
     "embed": {"from": ["title", "section", "content"], "model_config": {"model_name": ""}}},
    {"name": "project", "type": "string", "facet": true, "optional": true},
    {"name": "tags", "type": "string[]", "facet": true, "optional": true},
    {"name": "source", "type": "string", "facet": true},
    {"name": "created_at", "type": "int64"},
    {"name": "updated_at", "type": "int64"}
  ]
}`

func (s *Store) schema() (*api.CollectionSchema, error) {
	var sc api.CollectionSchema
	if err := json.Unmarshal([]byte(schemaJSON), &sc); err != nil {
		return nil, err
	}
	sc.Name = s.cfg.Collection
	for i := range sc.Fields {
		if sc.Fields[i].Embed != nil {
			sc.Fields[i].Embed.ModelConfig.ModelName = s.cfg.EmbedModel
		}
	}
	return &sc, nil
}

// Ensure creates the collection when it does not exist. Other collections
// on the same instance are never touched.
func (s *Store) Ensure(ctx context.Context) (created bool, err error) {
	cctx, cancel := context.WithTimeout(ctx, callTimeout)
	_, err = s.client.Collection(s.cfg.Collection).Retrieve(cctx)
	cancel()
	if err == nil || !isNotFound(err) {
		return false, err
	}
	s.log.Info("creating typesense collection (the first one on an instance downloads the embedding model)",
		"collection", s.cfg.Collection, "model", s.cfg.EmbedModel)
	sc, err := s.schema()
	if err != nil {
		return false, err
	}
	cctx, cancel = context.WithTimeout(ctx, createTimeout)
	defer cancel()
	if _, err := s.client.Collections().Create(cctx, sc); err != nil {
		return false, err
	}
	return true, nil
}

// Drop deletes the collection (reindex --wipe). A missing one is fine.
func (s *Store) Drop(ctx context.Context) error {
	cctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	_, err := s.client.Collection(s.cfg.Collection).Delete(cctx)
	if err != nil && !isNotFound(err) {
		return err
	}
	s.forgetAll()
	return nil
}

func isNotFound(err error) bool {
	var he *ts.HTTPError
	return errors.As(err, &he) && he.Status == http.StatusNotFound
}
