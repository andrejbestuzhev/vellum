package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/freema/vellum/internal/config"
	"github.com/freema/vellum/internal/vault"
)

// vaultCommand runs a command of the vault build when args name one;
// ok is false for anything else, which main handles as before.
func vaultCommand(args []string) (code int, ok bool) {
	if len(args) == 0 || args[0] != "reindex" {
		return 0, false
	}
	return runReindex(args[1:]), true
}

// runReindex rebuilds the Typesense collection from the vault:
// `vellum reindex [--wipe] [-vault DIR]`. It needs the TYPESENSE_*
// settings, whatever VAULT_SEARCH says.
func runReindex(args []string) int {
	fs := flag.NewFlagSet("reindex", flag.ContinueOnError)
	wipe := fs.Bool("wipe", false, "drop the collection and build it from scratch")
	dir := fs.String("vault", "", "vault folder (overrides VELLUM_VAULT_PATH)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	cfg := config.Load()
	path, err := resolveVaultPath(*dir, os.Getenv("VELLUM_VAULT_PATH"), false)
	if err == nil {
		err = ensureVaultDir(path, false)
	}
	if err != nil {
		logger.Error("reindex", "error", err)
		return 1
	}
	v, err := vault.New(path, vault.WithContextFile(cfg.ContextFile))
	if err != nil {
		logger.Error("reindex: open vault", "error", err)
		return 1
	}
	ix := vault.NewIndex(v)
	if err := ix.Build(); err != nil {
		logger.Error("reindex: read vault", "error", err)
		return 1
	}
	store, err := newStore(v, cfg.InboxDir, logger)
	if err != nil {
		logger.Error("reindex", "error", err)
		return 1
	}
	st, err := store.Reindex(context.Background(), ix, *wipe)
	if err != nil {
		logger.Error("reindex failed", "error", err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "reindex: %d notes, %d indexed, %d removed, %d failed, %d note ids in %s\n",
		st.Files, st.Indexed, st.Removed, st.Failed, st.InIndex, store.Collection())
	if st.Failed > 0 || st.InIndex != st.Files {
		return 1
	}
	return 0
}
