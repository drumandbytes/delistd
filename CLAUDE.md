# CLAUDE.md

## Project Overview

Delistd is a CLI and web tool that sends data removal requests to data brokers, oriented toward an EU/EEA user exercising GDPR Article 17 rights, with optional support for a second household profile. See [EU-NOTES.md](EU-NOTES.md) for the GDPR-specific setup and broker notes, and [README.md](README.md) for user-facing docs.

## Docs map

Read the doc relevant to what you're touching rather than everything up front.

- [docs/getting-started.md](docs/getting-started.md): plain-language setup guide for non-technical users (Gmail app password, wizard, troubleshooting). Keep it in step with the setup wizard wording.
- [docs/architecture.md](docs/architecture.md): tech stack, project structure, broker/template/send-flow concepts, web UI static assets.
- [docs/commands.md](docs/commands.md): CLI command reference and the `config.yaml` schema.
- [docs/code-patterns.md](docs/code-patterns.md): conventions and per-area gotchas. Read the matching section before touching sending, the reply classifier, the history store, the web server or browser automation.
- [docs/broker-replies.md](docs/broker-replies.md): reading broker email replies and turning each response type into the right `brokers.yaml` edit.
- [docs/auditing.md](docs/auditing.md): re-running the dead-code/security sweep and growing the broker lists.
- [docs/multi-profile.md](docs/multi-profile.md): the `--profile` / `profiles:` feature.
- [docs/ui-cli-parity.md](docs/ui-cli-parity.md): what the web UI and the CLI can each do, and what is deliberately one-sided.
- [docs/docker.md](docs/docker.md) and [docs/kubernetes.md](docs/kubernetes.md): the distroless image on `ghcr.io/drumandbytes/delistd` with a Chromium sidecar, and a single-pod manifest (keep to one replica, don't expose the UI, no NFS for `history.db`).
- [docs/outlook.md](docs/outlook.md) and [docs/oauth-app.md](docs/oauth-app.md): Outlook.com sign-in with OAuth, and the shared Microsoft app (registration, monitoring, rotation/abuse runbook).
- [docs/mcp.md](docs/mcp.md): the experimental `delistd mcp` agent server.
- [docs/stability.md](docs/stability.md): what a major version promises not to break (config keys, CLI flags, `history.db`, `brokers.yaml`, export JSON). Check before renaming or removing any of them.
- [docs/upgrading-to-3.0.md](docs/upgrading-to-3.0.md): what 3.0 removed and what to change.

## Quick Start

```bash
go build -o delistd ./cmd/delistd
go vet ./...
go test ./...
```

See [docs/commands.md](docs/commands.md) for the full CLI reference.

## Conventions

- **Commits**: Conventional Commits with a scope: `feat(brokers):`, `fix(site):`, `fix(audit):`, `chore(ci):`, `docs:`. [release-please](.release-please-manifest.json) reads them to bump the version and write `CHANGELOG.md`; an unscoped or non-conventional message just won't appear in the changelog. No `Co-Authored-By` / attribution trailers.
- **Branches & PRs**: work on a branch, open a PR, squash-merge. `main` stays releasable; patch-level release PRs auto-merge.
- **Go code**: conventions and the gotchas per area (sending, the reply classifier, the history store and profile scoping, the web server, browser automation) are in [docs/code-patterns.md](docs/code-patterns.md); read the matching section before touching one. The web UI's CSS rules (no inline styles, no toolchain, class tests) are in [docs/architecture.md](docs/architecture.md#web-ui-static-assets).

## Recurring work

- **Acting on broker email replies** (the frequent one): classify the reply, make the matching `data/brokers.yaml` edit. [docs/broker-replies.md](docs/broker-replies.md).
- **Keeping `data/brokers.yaml` honest**: liveness audit, dedup, growing the list from registries, and keeping the public "1,000+" broker claim (README, site, setup page) true to the real count: [docs/auditing.md](docs/auditing.md).
- **Removing or merging a broker with a guide page** (`/brokers/<id>/`, curated categories only - see `curatedCategories` in `cmd/delistd/cmd_guides.go`): the next deploy drops the page and the URL 404s via `site/layouts/404.html`. If it was merged into or renamed to another entry, add `/brokers/<old-id>/ /brokers/<new-id>/ 301` to `site/layouts/index.redirects` (the Hugo template that generates Cloudflare Pages' `_redirects`, also the `/docs/` to latest-release 302; a `static/_redirects` would collide with it) so links and search results land on the surviving page. Without a `404.html`, Pages serves the homepage with a 200 for unknown paths.
- **Re-sending**: brokers re-list you continuously. `./delistd send` is safe to re-run: it resumes where it left off (25-day per-broker cooldown, `daily_send_limit` cap); `delistd schedule install` / `delistd auto` run it unattended ([docs/commands.md](docs/commands.md)). EU cadence and hand-sending are in [EU-NOTES.md](EU-NOTES.md).
