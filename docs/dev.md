# Developing the vault build

The vault build is vellum with `VAULT_SEARCH=typesense`: notes live in one
`inbox/` folder, a Typesense collection indexes them section by section, and
MCP gets `remember`, `recall` and `digest` in place of `search_notes`.
Typesense computes the embeddings itself (`ts/multilingual-e5-small`); the
server never calls a language model.

## Everything in Docker, one command

```sh
docker compose -f docker-compose.dev.yml up -d --build
curl -s localhost:8080/healthz
```

This starts vault-mcp and a Typesense of its own, both on localhost only,
without Traefik and without OAuth. Notes go to `./vault` (yours to edit: the
server runs as uid 1000, set `VAULT_UID`/`VAULT_GID` in `.env.dev` if yours
differs); `./testdata` is mounted read-only at `/testdata`.

The **first** start downloads the embedding model into the Typesense volume
(about 3 minutes, 450 MB). `/healthz` answers at once; `recall` fails until
the log says `typesense index in step with the vault`.

Settings: `cp .env.dev.example .env.dev` and edit. Ports default to 8080
(server) and 8108 (Typesense); change them with `VAULT_MCP_PORT` and
`TYPESENSE_PORT` when they are taken.

Try it over MCP:

```sh
claude mcp add --transport http vault-dev http://localhost:8080/mcp
```

Then ask Claude to remember something and to recall it in other words. A
file you put into `vault/inbox/` by hand is searchable within about 3
seconds.

Rebuild the index from the files:

```sh
docker compose -f docker-compose.dev.yml exec vault-mcp /vellum reindex --wipe
```

## Integration tests

They run against the Typesense of the dev compose (`localhost:8108`, key
`vault-dev-key`), each test in a collection of its own that is dropped
afterwards:

```sh
docker compose -f docker-compose.dev.yml up -d typesense
go test -tags integration ./internal/search/typesense/
```

`TYPESENSE_URL` and `TYPESENSE_API_KEY` point them elsewhere. The plain
`go test ./...` needs no Typesense.

## Without Docker for the server

Typesense in one container, the server from source over stdio:

```sh
docker run -d --name vault-typesense -p 127.0.0.1:8108:8108 \
  -v vault-typesense:/data typesense/typesense:30.2 \
  --data-dir /data --api-key vault-dev-key

export VAULT_SEARCH=typesense TYPESENSE_URL=http://localhost:8108 TYPESENSE_API_KEY=vault-dev-key \
  VELLUM_INIT_STRUCTURE=false   # one inbox folder, no projects/ or archive/
go run ./cmd/vellum --mcp-stdio -vault ./vault -init
```

Connect Claude Code to it over stdio (the client starts the process):

```sh
go build -o bin/vellum ./cmd/vellum
claude mcp add --scope user vault-dev \
  -e VAULT_SEARCH=typesense -e TYPESENSE_URL=http://localhost:8108 -e TYPESENSE_API_KEY=vault-dev-key \
  -e VELLUM_INIT_STRUCTURE=false \
  -- "$PWD/bin/vellum" mcp "$PWD/vault" -init
```

The binary is built from `./cmd/vellum` (upstream's command, so the fork
stays easy to rebase); the image calls it `/vellum`.

## Where the code is

- `internal/search/typesense/` — collection schema, section split, indexing,
  watcher, recall, digest, reindex, and a `vault.Searcher` for the REST
  search (regex, folder-scoped and empty queries fall back to vellum's scan).
- `internal/mcpserver/vault_*.go` — the three tools and their instructions.
- `cmd/vellum/vault.go`, `cmd/vellum/reindex.go` — setup and the `reindex`
  command; `main.go` only calls them.

vellum notices files changed outside it only when its index is read, so the
watcher runs the index's `Reconcile` every 500 ms and indexes a changed note
after 2 seconds of quiet. A write through the MCP tools is indexed at once.
