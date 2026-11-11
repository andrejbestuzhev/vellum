//go:build integration

package typesense_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/freema/vellum/internal/mcpserver"
	"github.com/freema/vellum/internal/search/typesense"
	"github.com/freema/vellum/internal/vault"
)

// rig is a vault served over MCP with the vault tools, indexed into a
// collection of its own on a live Typesense (docker-compose.dev.yml).
type rig struct {
	vault   *vault.Vault
	index   *vault.Index
	store   *typesense.Store
	session *mcp.ClientSession
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func newRig(t *testing.T) *rig {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	v, err := vault.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ix := vault.NewIndex(v)
	if err := ix.Build(); err != nil {
		t.Fatal(err)
	}
	cfg := typesense.Config{
		URL:        env("TYPESENSE_URL", "http://localhost:8108"),
		APIKey:     env("TYPESENSE_API_KEY", "vault-dev-key"),
		Collection: fmt.Sprintf("vault_it_%d", time.Now().UnixNano()),
		EmbedModel: env("TYPESENSE_EMBED_MODEL", typesense.DefaultEmbedModel),
	}
	store, err := typesense.New(cfg, v, "inbox", slog.New(slog.NewTextHandler(os.Stderr, nil)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Ensure(ctx); err != nil {
		t.Fatalf("typesense at %s: %v", cfg.URL, err)
	}
	t.Cleanup(func() { _ = store.Drop(context.Background()) })
	store.Watch(ctx, ix)

	server := mcpserver.New(mcpserver.Deps{
		Vault: v, Index: ix, Searcher: store.Searcher(vault.NewScanSearcher(v, ix)),
		Structure: vault.DefaultStructure(), Version: "test",
		Extend: mcpserver.VaultTools(store), ExtraInstructions: mcpserver.VaultInstructions,
	})
	st, ct := mcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ss.Close() })
	session, err := mcp.NewClient(&mcp.Implementation{Name: "it", Version: "0"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return &rig{vault: v, index: ix, store: store, session: session}
}

// call invokes a tool and decodes its structured result into out.
func (r *rig) call(t *testing.T, name string, args map[string]any, out any) {
	t.Helper()
	res, err := r.session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if res.IsError {
		t.Fatalf("%s failed: %v", name, res.Content[0].(*mcp.TextContent).Text)
	}
	raw, _ := json.Marshal(res.StructuredContent)
	if err := json.Unmarshal(raw, out); err != nil {
		t.Fatal(err)
	}
}
