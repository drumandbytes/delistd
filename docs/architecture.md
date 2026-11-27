# Architecture

## Tech stack

Go 1.26+ with Cobra for the CLI, SMTP for sending (`internal/email`; SendGrid and Resend were removed, see [auditing.md](auditing.md#dead-code)), SQLite for history (`modernc.org/sqlite`, pure Go), YAML config (`gopkg.in/yaml.v3`), chromedp for `fill`/`confirm`/CAPTCHA detection, the official Go MCP SDK over STDIO for `delistd mcp`, and chi + html/template + HTMX for the web UI. The website is Hugo.

## Project structure

```
cmd/delistd/       main.go wires the root command and shared helpers; one cmd_*.go per command
internal/
  broker/          Broker struct, YAML loading, filtering, add/remove
  browser/         chromedp: form filling, CAPTCHA detection, confirmation links, the MCP
                   automation session, and the broker-domain allowlist (domain.go)
  config/          config.go (schema, Load/Save, validation), profiles.go, mail.go (email,
                   inbox, SMTP), providers.go (mail presets), secret.go (password encryption)
  email/           SMTP sender; removal.go renders + sends + builds the history record;
                   batch.go runs a list of them (used by the CLI send loop and the web job)
  history/         SQLite: requests, replies, tasks, pipeline status, per-profile scoping
  inbox/           IMAP scan + reply classification (rules.go, classify.go, response.go)
  evidence/        the export report (requests, replies, deadlines, authority)
  schedule/        unattended cycles: shared lock, state file, launchd/systemd install
  mcpserver/       profile-scoped MCP tools, reviewed batches, persistent browser sessions
  oauth/           Outlook.com device-code sign-in
  template/        email template rendering (templates/: gdpr, ccpa, generic)
  web/             server.go + routes.go; handlers_*.go by resource; pages*.go build page
                   data; templates/ and static/ are embedded
  bounces, country, dpa, datadir, fsutil, reclassify   small helpers
  imaptest, smtptest, testenv                          test-only servers and helpers
data/              brokers.yaml (1,000+, embedded via embed.go), brokers-verified.yaml,
                   authorities.yaml
scripts/           covgate (CI coverage gate), docs-versions, import-registries,
                   stylesnap (CSS regression tool), a11y/ (axe scan)
site/              the Hugo site (delistd.dev)
docs/              reference docs; EU-NOTES.md has the GDPR notes
```

## CI and releases

`ci.yml` runs `go build`, `go vet`, `go test -race` and `golangci-lint` (`.golangci.yml`) on every push and PR, with `zizmor` and `actionlint` when workflows change, plus coverage (`scripts/covgate`) and an accessibility scan. `release-please.yml` keeps an open release PR (version bump and `CHANGELOG.md`) from Conventional Commits on `main`; merging it tags `vX.Y.Z` and fires `release.yml`: GoReleaser cross-compiles `linux/darwin/windows x amd64/arm64` (`CGO_ENABLED=0`) and attaches archives and checksums, and a `docker` job pushes the distroless image to `ghcr.io/drumandbytes/delistd` (`vX.Y.Z`, `X.Y`, `latest`, with provenance and SBOM) and smoke-tests it per architecture. The smoke jobs alarm without gating the release. `main.version` is set by `-ldflags`; it is `dev` otherwise. `deploy.yaml` builds and publishes the site.

## Key concepts

### MCP automation

`cmd_mcp.go` exposes `internal/mcpserver` through the official Go MCP SDK. The
server is local and serves one client session. The client supplies the AI.
`begin_batch` and `retry_action` work only after a person approves a stored
preview (`batch_approvals`: single use, expiring, bound to the preview's digest)
on the web UI's Automation page or with `delistd mcp approve`; the server then
enforces the resulting profile/task/broker scope. Changing settings invalidates
that scope.

Browser tools use `internal/browser/automation.go`, retaining Chrome between
calls and referencing controls from fresh snapshots in an isolated JavaScript
world. Destinations are the selected broker's catalog hosts and their `www.`
form, nothing broader; on shared platforms (`sharedPlatforms` in `automation.go`)
page loads also stay under the catalog entry's form path, and a response from a
non-public address stops the session. Request interception checks redirects and
form traffic; a cumulative document CSP blocks popups, workers, and out-of-scope
connections. This deliberately limits
compatibility with embedded forms and external dependencies.

`internal/history/automation.go` adds an action journal. An action is claimed
before external work; interrupted submissions remain unknown and cannot be
automatically replayed. Notes, screenshots, and completion use `pending_tasks`,
which gains `actor` (`human` or `mcp:<client>`) and `review_reason`;
`removal_requests` gains `pipeline_actor`. Accepted requests remain awaiting a
response in the pipeline. Email uses `email.SendRemoval` and the existing
profile-scoped limits; inbox replies use the existing classifier with an
explicitly resolved owner. See [mcp.md](mcp.md) for operational limits.

### Broker
`data/brokers.yaml` is compiled into the binary (`data/embed.go`, `//go:embed`).
`broker.Load(override)` resolves it from `--brokers`, then `~/.delistd/brokers.yaml`
(written by `delistd update-brokers`, a conditional download), then the embedded
copy; no implicit `./data` scanning. `add-broker`/`cleanup-bounces` write to a
real path via `resolveBrokerWritePath()`.

`data/brokers-verified.yaml` is a second embedded list: a smaller set built from
public registries and authoritative privacy notices, where every entry is a
confirmed data broker with a current first-party privacy email or rights portal.
The send-family commands (`send`, `draft`, `mark-sent`, `serve`) call
`broker.LoadList(flag, cfg.Options.BrokerFile, cfg.Options.BrokerList)` instead of
`broker.Load`, adding two steps ahead of the `~/.delistd` fallback: an
`options.broker_file` path, then the verified list when `options.broker_list`
(or `send --list`) is `verified`. Everything else stays on the full list. Grow
the verified list with `scripts/import-registries`; see
[auditing.md](auditing.md).

Each broker in `data/brokers.yaml` (top-level key `brokers:`) has:
- `id`: Unique lowercase hyphenated identifier (e.g., `spokeo`, `been-verified`)
- `name`: Display name
- `email`: Privacy/removal contact email (may be empty string; see below)
- `website`: Company website (optional)
- `opt_out_url`: Direct opt-out link (optional)
- `region`: `us`, `eu`, or `global`
- `category`: `people-search`, `marketing`, `background-check`, `financial-b2b`, `data-intermediary`, `device-id-only` (tracks by cookie/device ID, not name/email, can't be reached via this tool's standard profile-based request), or `requires-id` (won't act on a request without a government-issued ID document or similar heavyweight identity verification; this tool won't supply one on your behalf)
- `countries`: Optional list of upper-case ISO 3166-1 alpha-2 codes (e.g. `[DE]`). When set, the default selection in `send`, `draft`, `mark-sent`, the web UI (list, send, email page) and the MCP server only includes the broker for profiles whose `country` is one of them. An empty or unrecognised profile country never hides a broker, and naming a broker with `send --broker` bypasses the scope (for example when you have a past address in that country). `list-brokers` shows the scope and the web list marks scoped brokers and says how many are hidden. For brokers that only hold one market's data and ask not to be sent requests from elsewhere.
- `notes`: Free-text, optional, used to record why an entry looks unusual (e.g. "use the form, not email")

**Brokers with `email: ""`** can't be reached by `send` at all; they only take requests through a web form/DSR portal, or the address bounced and was cleared by `cleanup-bounces`/`mark-bounced`. Use `delistd list-brokers --missing-email` to see them; they need manual follow-up outside the tool.

### Templates
Three email templates (`internal/template/templates/`):
- **GDPR**: Invokes EU Article 17 "Right to Erasure", the correct default for EU users
- **CCPA**: Invokes California Consumer Privacy Act
- **Generic**: General privacy request referencing multiple laws

### The website

`delistd guides` writes `site/data/brokers.json` and `guides.json` (broker facts, editorial text and the request letters rendered with the real templates); Hugo builds the directory and the per-broker guide pages from them (`site/content/brokers/_content.gotmpl`, `site/layouts/partials/guide-body.md`). `deploy.yaml` runs both. See [site/README.md](../site/README.md).

### Web UI static assets
`internal/web/static/` (embedded via `//go:embed static/*`) holds:
- `js/htmx.min.js`: vendored as-is.
- `css/tokens.css`: design tokens for both themes: colour and elevation, the type scale
  (`--fs-*`, in rem) and the corner radii (`--radius-*`). The only place raw colour values
  and font sizes are named. Linked first in `layout.html`.
- `css/app.css`: the component classes (`.btn`, `.card`, `.badge`, `.nav-link`, ...), the
  status tints (`.tint-*`, `.bg-*`, `.border-*`) and the page layouts, written against the tokens.
- `css/utilities.css`: a small, hand-written reset plus the layout / spacing / sizing /
  flex / grid utilities the templates use (`.p-4`, `.flex`, `.gap-2`, ...). There is no
  Tailwind or any other CSS toolchain: add a rule by hand when a template needs one, and
  size it from the scale in `tokens.css`. No inline `style=` attributes: name a class.

`css_utilities_test.go` keeps the three in step: it fails on a utility class a template
uses without a rule, and on a rule nothing uses. `utilities.css` is linked *after*
`app.css` on purpose: same-specificity rules are decided by source order, and a utility
like `.w-auto` has to win over a component class like `.input { width: 100% }`.

Before a CSS or template refactor, record the computed style of every element on every
page with `go run ./scripts/stylesnap` (see its header for the three commands), then
record again afterwards and `-compare` the two: it lists exactly which properties changed
on how many elements, which is how a dropped rule shows up. It needs Chrome and a running
`delistd serve` on demo data, so it is a local tool, not a CI step.

### Send flow
1. Load `~/.delistd/config.yaml` and resolve the active profile ([multi-profile.md](multi-profile.md)).
2. Load the broker list, then narrow it by region, category, exclusions and the profile's country (`broker.Select`, `broker.ForCountry`).
3. Keep brokers that match the `--status` filter (`history.MatchesFilter`, shared with the web Brokers page) and order them never-sent first, then oldest send (`history.SortBySendPriority`).
4. Cap the list at `options.daily_send_limit` per rolling 24h.
5. `email.Batch` renders each email, sends it over SMTP, stores the result in SQLite tagged with the profile, paces the sends, and stops after three authentication failures in a row. The CLI `send` and the web "Send all" job both run on it.

`send` skips brokers emailed successfully in the last 25 days (per profile), so re-running is always safe.
