# Code Patterns

## Conventions

- **Errors**: wrap with context, `fmt.Errorf("context: %w", err)`. Never pass a variable as the format string (`fmt.Errorf(someVar)`): `go vet` flags it, and it is a real bug if the value can contain a `%`.
- **Config**: YAML with struct tags; defaults are applied in `config.Load`. `Options.Template` defaults to `gdpr`, not `generic`, because the tool is aimed at GDPR Article 17 use; an explicit `template:` always wins.
- **Commands**: one `cmd/delistd/cmd_*.go` per command. Flags live in the `*Cmd()` constructor, the logic in a separate `run*()` so it is testable without Cobra. `main.go` only holds root wiring and helpers shared by three or more commands. Flags several commands share (`--region`, `--category`) have one helper (`addRegionCategoryFlags`).
- **Regexes**: compile once at package level, never inside a per-email or per-loop function. The classifier builds its phrase lists once from the small DSL in `internal/inbox/rules.go`.
- **Sending**: a new send path goes through `email.SendRemoval` for render + send + record, and through `email.Batch` if it sends a list, so it inherits the daily budget, the auth-failure cutoff and the pacing. Add only your own policy on top.
- **Mutating shared config in the web UI**: `Server.config` is an `atomic.Pointer`. Read with `s.getConfig()`; to write, copy (`newCfg := *cfg` or `cfg.Clone()` when passwords change), mutate the copy, then `s.config.Store(&newCfg)`. Never mutate what `getConfig()` returned: another goroutine may hold that pointer.
- **Class names**: the web UI has no inline styles and no CSS toolchain; see [architecture.md](architecture.md#web-ui-static-assets). `TestTemplateUtilityClassesHaveRules` and `TestStylesheetClassesAreUsed` fail when markup and stylesheets drift apart, so add a rule and its use together. Class strings built in Go must be whole class names, so the tests can see them.

## Reply classifier

`internal/inbox/rules.go` holds the keyword phrases (English, CCPA, GDPR, German, French) in a DSL where a space matches `\s+` and `_` matches `\s*`; `classify.go` scores them. Add new patterns there and keep non-English ones high-signal so they don't match English text. A `rejected`, `success` or `no_data_held` verdict below 0.7 confidence is flagged `NeedsReview`, because a wrong one silently drops a broker from `delistd export`'s overdue list. Two golden corpora (`classifier_golden_test.go`, `parser_golden_test.go`) pin the behaviour; a change that moves a verdict must be deliberate and regenerate the golden.

## History store

- Composite indexes (including `(profile_id, broker_id, sent_at DESC)`) serve the "latest record per broker, per profile" query used throughout. Times are stored as UTC text (SQLite compares them as strings, so a local offset would break the ordering), and list queries tie-break on `id DESC` so a coarse clock can't reorder them.
- `NewStore` opens the DB in WAL mode with `busy_timeout(5000)` so `delistd serve` and a CLI run can write at once. A backup of `history.db` must include the `-wal` file (or be taken with the store closed), and WAL does not work on network filesystems.
- CLI commands open the store through `openHistoryStore`, which runs the versioned, transactional saved-reply migration (`inbox.MigrateNoDataResponses`). Keep new entry points on it so upgrades don't need a visit to the web UI.
- **Profile scoping**: a `history.Store` method that reads or writes per-user data takes `profileID string` first and filters by it ([multi-profile.md](multi-profile.md)). The deliberate exceptions are `ClearBrokerResponses` and `GetAllBrokerResponses`, which work on the one shared inbox. The by-ID methods (`GetPendingTaskByID`, `CompletePendingTask`, `MarkTaskOpened`, `UpdateBrokerResponseClassification`, `UpdateBrokerResponseBody`) enforce the profile in the query: the getter returns `(nil, nil)` for another profile's ID, and the updaters are silent no-ops. To know whether an update landed, re-fetch.
- `UpdatePipelineStatus` only moves a request forward (ranks in `pipeline_rank.go`), so a late "awaiting response" can't undo a confirmation. A person's decision in the UI goes through `OverridePipelineStatus`, which always applies. Both record who acted in `pipeline_actor`.
- `ConsumeBatchApproval` is the only way to spend an MCP batch approval and it is atomic: one approval, one batch.
- `no_data_held` appears in the export JSON (`response_type`, `pipeline_status`), so it can be added to but never renamed. `MigrateNoData` only relabels saved replies; resolving a request is `ResolveNoData`'s job.
- Pipeline counts and pending rows share `history.GetActionItems`; use it for the Pipeline and Action Needed views together.

## Web server

- It binds `127.0.0.1` by default: a single-user local tool with no login, where a `0.0.0.0` listener would expose one person's data and send controls to the network. The exception is `serve --listen`, for the Docker image only (see [docker.md](docker.md)); the bare `delistd` launch never takes it. `requireAllowedHost` rejects non-loopback Host headers to stop DNS rebinding unless `--allowed-hosts` or `DELISTD_ALLOWED_HOSTS` allow them. The image defaults that to `lan,local` (private IPs and local-only names, never a public hostname): don't let it become `*`, which switches the guard off, and add tests for any new pattern in `isHostAllowed`.
- CSRF protection is the standard library's `http.CrossOriginProtection` (Go 1.25+), middleware in `routes.go`. It checks `Sec-Fetch-Site` and `Origin`: no tokens, no cookie. Templates still render `{{.CSRFField}}`, a harmless stub; don't hand-write a `csrf_token` input (one once 403'd every click on a page). Non-browser clients with neither header are allowed through by design.
- `getOrCreateSession` also calls `r.AddCookie` for the session it just created. The wizard's `handleSetupProfile` creates then updates a session in one request, and without that the update silently no-ops and a fresh install loops back to step one.
- `JobManager` evicts finished jobs after `jobRetention` (7 days) when the next job starts, and jobs are scoped per profile (`Job.ProfileID`), so two profiles can have a send job at once.
- Partials in `templates/partials/` are named sub-templates: `{{template "partials/x.html" .}}` resolves, and `(dict "Key" val ...)` passes several values. They are also standalone templates for HTMX fragments (`renderPartial`). Prefer `{{template}}` over copying a partial's markup. And never write `{{` inside a prose comment in a `.html` template: it is parsed as a real action and fails with a confusing "unexpected EOF" reported against a different template.

## Browser automation

- Every URL that comes from outside (a parsed email, a form) goes through `matchesAllowedDomain` in `internal/browser/domain.go`, the one place the allowlist logic lives (`NavigateAndFill` and the confirmation handler's redirect-hop check both call it). Route a new fetching or navigating path through it too. An empty allowlist means "none configured" and passes.
- The MCP automation session's document CSP (`restrictDocument` in `automation.go`) works only because of two Chrome details: a document is paused again at the response stage only if its `ContinueRequest` sets `InterceptResponse`, and Chrome applies just the `sandbox` part of a CSP edited with `Fetch.continueResponse`. So the response is fulfilled with its own body and the new header. `TestInjectedCSPApplies` runs a real page against it.
- Screenshots are named by their real format: Chrome returns JPEG below quality 100 and PNG at 100.
- Chrome starts on the browser's own context (`Browser.start`), not a per-call timeout context, which would close it after one form.

## Accessibility

CI runs axe-core (WCAG 2.2 A/AA, light and dark) over the website and the web UI on fictional seed data; `scripts/a11y/run.sh` does the same locally (needs Go, Hugo, Node and Chrome). `scripts/a11y/probe.mjs <chrome> <url>...` covers what axe can't (skip link, tab stops and focus rings, mouse-only controls, 320px reflow); it is informational. Add a new web UI page to its URL list. Rules it enforces: every `<select>` and icon-only control has an `aria-label`, links in body text are underlined, text reaches 4.5:1 in both themes, and headings step down one level at a time. Use `--accent` for text and borders and `--accent-solid` for fills behind white text.
