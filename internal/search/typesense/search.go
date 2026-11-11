package typesense

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/typesense/typesense-go/v3/typesense/api"
)

// rawHit is a search hit decoded from the response body. The typed client
// result has no hybrid_search_info, which carries the fusion score.
type rawHit struct {
	Document Doc `json:"document"`
	Hybrid   struct {
		Score float64 `json:"rank_fusion_score"`
	} `json:"hybrid_search_info"`
}

// search runs one search request on the collection.
func (s *Store) search(ctx context.Context, params *api.SearchCollectionParams) ([]rawHit, error) {
	cctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	res, err := s.raw.SearchCollectionWithResponse(cctx, s.cfg.Collection, params)
	if err != nil {
		return nil, fmt.Errorf("typesense search: %w", err)
	}
	if res.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("typesense search: %s: %s", res.Status(), res.Body)
	}
	var body struct {
		Hits []rawHit `json:"hits"`
	}
	if err := json.Unmarshal(res.Body, &body); err != nil {
		return nil, fmt.Errorf("typesense search: %w", err)
	}
	return body.Hits, nil
}

func ptr[T any](v T) *T { return &v }
