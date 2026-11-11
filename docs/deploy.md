# Deploying the vault build on the shared infra

`docker-compose.vault.yml` runs vault-mcp only. Typesense, Traefik and the
`db` / `traefik_public` networks belong to the shared infra and must exist
first. vault-mcp writes to its own collection (`vault_sections`) and never
touches the other collections on the instance.

## Typesense requirements

- Typesense 30.x reachable on the `db` network as `http://typesense:8108`.
- **Memory: at least 1 GiB for the Typesense container.** The embedding
  model alone takes about 615 MiB once loaded (measured on 30.2: 59 MiB
  before the model, 674 MiB with the model and an empty collection, 699 MiB
  with 1,000 documents). A 512 MiB limit gets the container killed when the
  collection is created.
- Outbound HTTPS from Typesense to `huggingface.co` the first time: the
  model is downloaded into its data volume (about 3 minutes, 450 MB).

## API key

Use a key scoped to the vault collection, so a leaked key cannot read or
change anything else. Create it with the instance's admin key:

```sh
docker run --rm --network db alpine:3.20 wget -qO- \
  --header="X-TYPESENSE-API-KEY: $ADMIN_KEY" --header="Content-Type: application/json" \
  --post-data='{"description":"vault-mcp","actions":["*"],"collections":["vault_sections"]}' \
  http://typesense:8108/keys
```

The `value` of the answer is `TYPESENSE_API_KEY`. It can create, fill and
drop `vault_sections` (needed by `reindex --wipe`) and gets 401 for every
other collection.

## First deployment

```sh
git clone -b vault <fork url> /opt/apps/vault-mcp
cd /opt/apps/vault-mcp
cp .env.prod.example .env && chmod 600 .env   # fill in host, secret, Typesense key
mkdir -p vault && sudo chown 65532:65532 vault   # the image runs as uid 65532
git checkout --detach && ./scripts/vault-deploy.sh "$(git rev-parse --short HEAD)"
```

The log (`docker compose -f docker-compose.vault.yml logs -f`) shows
`typesense collection created` (or that it exists) and then `typesense
index in step with the vault`. `https://<host>/healthz` answers as soon as
the server listens and reports the deployed commit as its version.

## Continuous deployment

Every push to the `vault` branch deploys it
(`.github/workflows/vault-deploy.yml`): `go vet` and the tests run first,
then the workflow connects over SSH, checks the pushed commit out in
`/opt/apps/vault-mcp` and runs `scripts/vault-deploy.sh` there (build, up,
health check, prune). Pushes queue: one deploy runs at a time. To deploy a
commit again, re-run its run (Actions → vault-deploy → Re-run jobs); there
is no manual trigger, as GitHub offers one only for workflows on the
default branch, and the fork's default branch mirrors upstream.

Repository secrets (Settings → Secrets and variables → Actions):

| Secret | Value |
|---|---|
| `SSH_HOST` | the server |
| `SSH_USER` | the deploy user; it needs docker and write access to `/opt/apps/vault-mcp` |
| `SSH_PRIVATE_KEY` | a private key whose public half is in that user's `authorized_keys` |
| `SSH_FINGERPRINT` | SHA256 fingerprint of the host key the client will pick: with both ECDSA and ED25519 keys on the server that is ECDSA, `ssh-keygen -lf /etc/ssh/ssh_host_ecdsa_key.pub -E sha256` |

By hand on the server:
`cd /opt/apps/vault-mcp && git fetch && git checkout --force --detach origin/vault && ./scripts/vault-deploy.sh "$(git rev-parse --short HEAD)"`.
A failed deploy prints the command that rolls back to the previous one.
At start the server indexes what changed while it was down.

## Reindex

The collection is a cache of the files and can be rebuilt any time:

```sh
docker compose -f docker-compose.vault.yml exec -T vault-mcp /vellum reindex          # re-upsert all, drop gone notes
docker compose -f docker-compose.vault.yml exec -T vault-mcp /vellum reindex --wipe   # drop the collection first
```

It prints the number of notes and of distinct note ids in the collection;
they must match. A difference is logged as a warning and the command exits
with 1. `--wipe` is needed after changing `TYPESENSE_EMBED_MODEL`; recall
returns nothing until it finishes.

Nightly, from the host's crontab (`crontab -e`):

```cron
30 4 * * * cd /opt/apps/vault-mcp && docker compose -f docker-compose.vault.yml exec -T vault-mcp /vellum reindex >> /var/log/vault-mcp-reindex.log 2>&1
```

## Connect Claude

The connector URL must end with `/mcp`.

Claude Code:

```sh
claude mcp add --scope user --transport http vault https://<host>/mcp
```

A browser opens the consent screen; enter `VELLUM_CLIENT_SECRET` and
approve.

claude.ai: Settings → Connectors → Add custom connector, URL
`https://<host>/mcp`. Either leave the OAuth fields empty and enter the
secret on the consent screen, or put `VELLUM_CLIENT_ID` and the secret into
the advanced settings.

## Environment

| Variable | Default | Meaning |
|---|---|---|
| `VAULT_SEARCH` | unset | `typesense` turns the vault build on (set by the compose). Unset: plain vellum. |
| `TYPESENSE_URL` | — | Typesense base URL, `http://typesense:8108` on the shared infra. |
| `TYPESENSE_API_KEY` | — | Scoped key, see above. |
| `TYPESENSE_COLLECTION` | `vault_sections` | Collection name. |
| `TYPESENSE_EMBED_MODEL` | `ts/multilingual-e5-small` | Built-in Typesense model. |
| `VAULT_MCP_HOST` | — | Host Traefik routes to the server. |
| `VELLUM_ISSUER_URL` | — | `https://<host>`, the OAuth issuer. |
| `VELLUM_CLIENT_ID` / `VELLUM_CLIENT_SECRET` | `vellum` / — | OAuth client; the secret is at least 32 characters. |
| `VELLUM_INBOX_DIR` | `inbox` | The one folder notes are written to. |

Everything else is vellum's own configuration, see `.env.example`.
