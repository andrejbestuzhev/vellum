package typesense

import (
	"log/slog"
	"net/http"
	"sync"
	"time"

	ts "github.com/typesense/typesense-go/v3/typesense"
	"github.com/typesense/typesense-go/v3/typesense/api"

	"github.com/freema/vellum/internal/vault"
)

// callTimeout bounds one Typesense request. Indexing a note embeds its
// sections inside Typesense, which takes longer than a search.
const callTimeout = 30 * time.Second

// Store is the Typesense side of the vault: it indexes notes and runs
// queries. Safe for concurrent use.
type Store struct {
	cfg    Config
	raw    *api.ClientWithResponses // for responses the typed client drops fields of
	client *ts.Client
	vault  *vault.Vault
	inbox  string
	log    *slog.Logger

	mu      sync.Mutex
	indexed map[string]string // path -> hash of the content last indexed
}

// New connects a Store to Typesense. Nothing is sent until the first call;
// run Ensure before indexing.
func New(cfg Config, v *vault.Vault, inbox string, log *slog.Logger) (*Store, error) {
	raw, err := api.NewClientWithResponses(cfg.URL,
		api.WithHTTPClient(&http.Client{}), // deadlines come from each call's context
		api.WithAPIKey(cfg.APIKey))
	if err != nil {
		return nil, err
	}
	return &Store{
		cfg:     cfg,
		raw:     raw,
		client:  ts.NewClient(ts.WithAPIClient(raw)),
		vault:   v,
		inbox:   inbox,
		log:     log,
		indexed: map[string]string{},
	}, nil
}

// Collection is the name of the collection the Store writes to.
func (s *Store) Collection() string { return s.cfg.Collection }

func (s *Store) docs() ts.DocumentsInterface {
	return s.client.Collection(s.cfg.Collection).Documents()
}

// isIndexed reports whether path was last indexed with this content hash.
func (s *Store) isIndexed(path, hash string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.indexed[path] == hash
}

func (s *Store) markIndexed(path, hash string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if hash == "" {
		delete(s.indexed, path)
		return
	}
	s.indexed[path] = hash
}

func (s *Store) forgetAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.indexed = map[string]string{}
}
