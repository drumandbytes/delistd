# Contributing to Delistd

The most helpful things:

- **Brokers**: `data/brokers.yaml` can always use more entries, and contact details go stale. Not up for a PR? [Report a broker](https://github.com/drumandbytes/delistd/issues/new?template=broker.yml) (new, changed, dead, or an unusual reply).
- **Mail providers**: a preset for a provider the setup wizard lacks, or a report that one doesn't work: [open a mail provider issue](https://github.com/drumandbytes/delistd/issues/new?template=mail_provider.yml) or see [Adding a mail provider](#adding-a-mail-provider).
- **Privacy authorities**: `data/authorities.yaml` isn't exhaustive and links move.
- **Templates, bug fixes, documentation.**

Open an issue first for anything non-trivial (a feature, a refactor, a behaviour change) so we can agree on the approach. Small, obvious fixes (a typo, a dead link, a one-line bug) can go straight to a pull request.

## Development setup

Needs [Go](https://go.dev/dl/). Then:

```bash
go build -o delistd ./cmd/delistd
go vet ./...
go test ./...
```

[docs/architecture.md](docs/architecture.md), [docs/commands.md](docs/commands.md) and [docs/code-patterns.md](docs/code-patterns.md) cover the layout, the commands, and the conventions and gotchas worth reading before touching sending, the reply classifier, the history store or the web UI.

Web UI styling has no toolchain and no inline styles; two tests keep templates and stylesheets in step ([architecture.md](docs/architecture.md#web-ui-static-assets)). Before a CSS or template refactor, record the pages with `go run ./scripts/stylesnap` and compare afterwards, so a dropped rule shows up as a changed property instead of a surprise.

### Screenshots

The images in the README and `docs/` come in light and dark pairs (`<name>-light.png`, `<name>-dark.png`) and are shown with `<picture>` (the website's dashboard copies are in `site/static/img`) so GitHub picks the one matching the reader's theme. Regenerate them with `scripts/a11y/doc-images.sh` (needs Go, Node and a Chromium-based browser; pass its binary as the first argument if it isn't Chrome). It runs the app against the fictional seed data from `scripts/a11y/seed`, so no real data ends up in an image. To add a screen, add it to `scripts/doc-images/capture.mjs` and embed both files with a `<picture>` block.

## Adding or updating a broker

`data/brokers.yaml` is compiled into the binary. Each entry:

```yaml
- id: example-broker
  name: Example Broker
  email: privacy@example.com
  website: https://example.com
  opt_out_url: https://example.com/optout
  region: us  # us, eu, or global
  category: people-search  # people-search, marketing, background-check, financial-b2b, data-intermediary, device-id-only, or requires-id
```

`./delistd add-broker` does this interactively. Before opening a PR, run:

```bash
go run ./cmd/delistd validate-brokers
```

It checks ids, names, regions, emails and URLs, and rejects a list under 200 entries (a sign it was truncated). If you're editing an entry because a broker answered a real request, [docs/broker-replies.md](docs/broker-replies.md) says which reply types need `email: ""`, a `notes:` line, or removal.

## Adding a mail provider

Delistd speaks plain SMTP and IMAP to any provider; presets in the wizard, `delistd init` and `delistd profile` mean users only type an address and an app password. A preset is one entry in `Providers` in [`internal/config/providers.go`](internal/config/providers.go):

```go
{ID: "example", Name: "Example Mail", SMTPHost: "smtp.example.org", SMTPPort: 465, IMAPHost: "imap.example.org", IMAPPort: 993,
	HelpURL: "https://example.org/help/app-passwords", Note: "Create an app password with SMTP + IMAP access."},
```

- There's no TLS setting: ports 465/993 use TLS from the start, any other port must offer STARTTLS. Hosts on localhost (bridges like Proton's) skip certificate verification.
- Leave `IMAPHost` empty for send-only services (like Amazon SES); replies then go to a separate inbox.
- If you can't test the preset with a real account, put "unverified" in its `Name` and ask for reports in its `Note` (the Proton preset did until a user confirmed it). Remove the label once someone confirms it works.
- Keep `custom` last. The CLI menu is numbered in slice order, so if you insert before an existing entry, update `cmd/delistd/mail_prompt_test.go`.
- Add a row to the provider table in the README ("Email account").
- OAuth-only providers can't be a plain preset (the CLI wizard asks for a password). Outlook.com has its own `OutlookOAuth` entry, added to the web dropdown by `providersForForm` and used by `delistd oauth login` ([docs/outlook.md](docs/outlook.md)).

Before opening the PR, send a real test email with your own account from the web setup wizard's "Test Your Setup" step, and mention in the PR that you did.

## Commits and pull requests

- [Conventional Commits](https://www.conventionalcommits.org/) with a scope, `feat(brokers):`, `fix(site):`, `fix(audit):`, `chore(ci):`, `docs:`. [release-please](.release-please-manifest.json) reads these to version releases and write `CHANGELOG.md`; an unscoped or non-conventional message just won't show up there.
- Work on a branch, open a PR against `main`. PRs are squash-merged, so the PR title becomes the commit message, make it a clean Conventional Commit line.
- All CI checks must pass: `go build`, `go vet`, `go test -race` and lint on Linux, the tests on macOS, Windows and Linux arm64, `govulncheck`, a build of the docs site, and a coverage gate. The gate enforces `codecov.yml`: total coverage stays above its floor (88%) and at least 80% of the statements you changed are covered (PRs that change fewer than 10 statements are reported, not gated). Run it locally with `go test -coverprofile=coverage.out ./... && go run ./scripts/covgate -profile coverage.out -base origin/main`.
- A `PR title` check warns when the title isn't a Conventional Commit. It's advisory, but the title decides the release version, so fix it before merging.
- Update documentation and tests alongside the code.

A single maintainer reviews every PR; expect a first response within a few days.

## Reporting a security issue

See the org-wide [Security Policy](https://github.com/drumandbytes/.github/blob/main/SECURITY.md), please don't open a public issue for a vulnerability. See the [Security](README.md#security) section of the README for how Delistd itself handles your credentials and personal data locally.

## License

Contributions are accepted under this repository's [MIT license](LICENSE).
