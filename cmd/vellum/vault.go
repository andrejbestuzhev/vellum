package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/freema/vellum/internal/mcpserver"
	"github.com/freema/vellum/internal/search/typesense"
	"github.com/freema/vellum/internal/vault"
)

// vaultHooks is what the Typesense vault search changes in the server:
// the searcher behind the REST search, and the MCP tools and instructions.
type vaultHooks struct {
	Searcher     vault.Searcher
	Extend       func(*mcp.Server, mcpserver.Deps)
	Instructions string
}

// setupVault turns on the Typesense vault search when VAULT_SEARCH is
// typesense: it starts the indexer (collection, catch-up, watcher) in the
// background and returns the hooks. Otherwise the server stays plain vellum
// and scan is returned unchanged. A bad Typesense configuration stops the
// process: asked for, the search must not silently be missing.
func setupVault(v *vault.Vault, ix *vault.Index, scan vault.Searcher, inbox string, logger *slog.Logger) vaultHooks {
	if !typesense.Enabled() {
		return vaultHooks{Searcher: scan}
	}
	store, err := newStore(v, inbox, logger)
	if err != nil {
		logger.Error("VAULT_SEARCH=typesense", "error", err)
		os.Exit(1)
	}
	logger.Info("vault search on typesense", "collection", store.Collection())
	go store.Run(context.Background(), ix)
	return vaultHooks{
		Searcher:     store.Searcher(scan),
		Extend:       mcpserver.VaultTools(store),
		Instructions: mcpserver.VaultInstructions,
	}
}

func newStore(v *vault.Vault, inbox string, logger *slog.Logger) (*typesense.Store, error) {
	cfg, err := typesense.ConfigFromEnv()
	if err != nil {
		return nil, err
	}
	return typesense.New(cfg, v, inbox, logger)
}
