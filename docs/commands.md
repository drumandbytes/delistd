# Commands & Configuration

```bash
go build -o delistd ./cmd/delistd   # build
go test ./...                       # test
```

## Commands

Every command except `profile`, `add-broker` and `list-brokers` accepts a global `--profile <id>`; omit it with one profile, and it is required once there are several ([multi-profile.md](multi-profile.md)). `--config <path>` and `--brokers <path>` are global too.

| Command | What it does |
|---|---|
| `delistd` / `delistd serve [-p 3000] [--listen 0.0.0.0] [--allowed-hosts lan,local]` | The web UI (what double-clicking the binary runs). `--listen` and `--allowed-hosts` are for containers and LANs ([docker.md](docker.md)) |
| `init` | Interactive config setup |
| `send [--dry-run] [--resend] [--ignore-daily-limit] [--manual] [--list full\|verified] [--broker id] [--region eu] [--category c] [--exclude id] [--status eligible\|never\|failed\|all]` | Send removal requests; `--manual` walks the list so you send each yourself |
| `draft [<broker-id>...] [--region] [--category] [-o dir]` | Render the emails, to stdout or one `.eml` per broker |
| `mark-sent <broker-id>... [--region] [--category] [--dry-run]` | Record a hand-sent request |
| `monitor [--days 7] [--watch] [--reclassify]` | Scan the inbox for replies; `--reclassify` re-runs the classifier over every saved reply |
| `status [--limit 50]` / `pipeline` | History and stats; which brokers need follow-up |
| `confirm` / `fill` | Click confirmation links from broker emails; fill opt-out forms with a browser. `DELISTD_CHROME_URL=http://host:9222` attaches to an existing Chrome ([docker.md](docker.md)) |
| `export [-o file] [--format html\|json] [--since 2026-01-01]` | Evidence report for a DPA or noyb complaint |
| `list-brokers [--region] [--category] [--search text] [--missing-email] [--all-countries]` | The broker list, minus brokers that only handle other countries than your profile's |
| `add-broker` | Add a broker of your own |
| `mark-bounced <broker-id>...` / `cleanup-bounces` | Correct the record when an email bounced; find and clear bounced addresses |
| `audit-brokers [--region] [--category] [--timeout 10] [--fail-on-dead] [--fix]` | MX and website liveness check; `--fix` clears dead email domains |
| `validate-brokers [file]` | Structural check (ids, names, regions, emails, URLs; non-zero exit on a problem); no argument checks both built-in lists |
| `update-brokers [--check]` | Fetch the latest list (conditional; writes `~/.delistd/brokers.yaml`) |
| `auto [--once] [--every 6h]` | Unattended cycle: send what is due for every profile, then scan inboxes |
| `schedule install\|remove\|status` | Have launchd or systemd run `auto --once` every 6 hours |
| `passphrase set\|clear` | Extra gate on stored mail passwords ([README](../README.md#security)) |
| `profile list\|add\|edit\|remove` | Named profiles |
| `oauth login [--client-id ID] [--email ADDRESS]` | Sign in to Outlook.com with a device code and store it as the sending and inbox account ([outlook.md](outlook.md)); needs an existing config. Uses Delistd's shared Microsoft app unless `--client-id`, a config `client_id` or `DELISTD_OAUTH_CLIENT_ID` says otherwise |
| `mcp [--demo] [--headless] [--chrome-path p]`, `mcp approve <id>`, `mcp pending` | Local MCP server over stdio (experimental); approve or list agent batches ([mcp.md](mcp.md)) |
| `guides [-o site/data]` | Maintainer: write the data the website builds its guide pages from |

**Broker list.** It is embedded in the binary. For `send`, `draft`, `mark-sent` and `serve` the order is: `--brokers <path>`, then `options.broker_file`, then `options.broker_list: verified` (the smaller registry-sourced list, `data/brokers-verified.yaml`), then `~/.delistd/brokers.yaml` (from `update-brokers`), then the embedded full list. `send --list full|verified` overrides `options.broker_list` for one run. Other commands (`audit-brokers`, `guides`, `list-brokers`, reply processing) always use the full list. `add-broker` and `cleanup-bounces` edit `./data/brokers.yaml` in a source checkout (or the `--brokers` file); an installed copy writes your own entries to `~/.delistd/brokers.local.yaml`, merged over whichever list is in use (same id: yours wins) and never touched by `update-brokers`.

**Selection flags.** `--region` and `--category` repeat or take commas on `send`, `draft`, `mark-sent`, `list-brokers` and `audit-brokers` (values OR within a flag, AND across flags); `send` also takes `--broker` and `--exclude`. `--status eligible` (the default) means never sent, failed, or last success at least 25 days ago.

**Automation.** `auto --once` runs one cycle; without it, `auto` loops every `--every` (minimum 1h) in the foreground and refuses to start while the OS job is installed. `schedule install` writes a launchd agent (`~/Library/LaunchAgents/com.drumandbytes.delistd.auto.plist`, output in `auto.log` beside the config) or a systemd user timer (`delistd-auto.timer`, output in the journal) that runs `auto --once --config <abs path>` at 00:07, 06:07, 12:07 and 18:07; missed slots run on wake. It is every 6 hours, not daily, because `daily_send_limit` is a rolling 24h window: a run exactly 24h later would find the cap still used up. All modes share `auto.lock` in the config directory, so cycles never overlap, and write the last result to `auto-state.json` (shown by `schedule status`).

The web UI's Settings → Automation card does the same without a terminal: install or remove the OS job, turn on the in-app scheduler (`schedule.enabled`: while `serve` is open it runs `auto --once` as a child process every 6 hours), or run a cycle now. It covers every profile. "Send all" refuses while a cycle holds the lock, and a job paused by the daily cap and resumed at startup skips brokers sent since it paused.

**Web UI equivalents.** `export` is History → Export evidence; `mark-bounced` is the "bounced?" link next to a Sent status on Brokers; `update-brokers` is Settings → Broker List (it also reloads the running server's list); `add-broker` is Brokers → + Add broker. Shared-list maintenance (`audit-brokers`, `validate-brokers`, `cleanup-bounces`, `guides`) and browser automation (`fill`, `confirm`) stay CLI-only. The full mapping: [ui-cli-parity.md](ui-cli-parity.md).

## Configuration

`delistd mcp` shares the web UI's configuration, broker list, profiles and history; batches start only after you approve them ([mcp.md](mcp.md)). It is experimental and outside the stability contract ([stability.md](stability.md)).

User config is stored at `~/.delistd/config.yaml` (see `config.example.yaml` for the full schema). Key sections:

- `profiles`: one entry per person: `id` (stable, stored in history, `default` for the one `delistd init` creates) + name/address/email + `additional_emails`/`name_variants`/`previous_addresses`/`additional_phones` for catching records indexed under old identities (editable in `delistd init` / `delistd profile edit` and on the web profile form, one per line). Each entry can set its own `mail.email`/`mail.inbox` to send/monitor through a dedicated account instead of the shared blocks below; see [multi-profile.md](multi-profile.md). `passphrase_protected` (top level) is set by `delistd passphrase set` and marks mail passwords as passphrase-protected. `country` is an ISO 3166-1 alpha-2 code (`init` and the web form accept a name and store the code). A 1.x config (top-level `profile:` block, free-text country, `email.provider`) is migrated on first load and the original is saved with passwords blanked
- `email`: `from` + `smtp` (`host`, `port`, `username`, `password`). TLS is on by default: port 465 = implicit TLS, any other port = mandatory STARTTLS; `use_tls: false` is only accepted without a username (a local relay). Loopback hosts (Proton Mail Bridge) skip certificate verification. `provider:` is optional (only `smtp` is accepted). Setup presets live in `internal/config/providers.go`
- `options`: `template` (`gdpr`/`ccpa`/`generic`, or `custom` to use your own `custom_template` text: Go template with the built-in fields, optional first line `Subject: ...`), `rate_limit_ms`, `daily_send_limit`, `broker_list` (`full`/`verified`), `broker_file` (path to your own list), `regions`, `excluded_brokers`, `excluded_categories` (skip every broker in a category, e.g. `requires-id`), `send_mode` (`manual` = Delistd never sends; render with `draft` / `send --manual`, record with `mark-sent`; no `email:` block needed)
- `email.smtp` / `inbox` also take `auth: oauth2` and `client_id` for Outlook.com, written by `delistd oauth login` (the refresh token is kept, encrypted, in `password`). See [outlook.md](outlook.md)
- `inbox`: IMAP settings (`provider` = a preset id that fills `server`/`port`, or set them directly; 993 = TLS, other ports = STARTTLS), for `monitor`/`pipeline`/the web UI's inbox scan. Shared by default across every profile that doesn't set its own `mail.inbox` override (see [multi-profile.md](multi-profile.md#shared-inbox)); `monitor` scans every distinct inbox in one run, the web UI's scan/rescan only the active profile's own
- `pipeline`: browser automation settings for `fill`
- `schedule`: `enabled: true` makes a running `delistd serve` run an automated cycle every 6 hours (the in-app fallback when the OS job from `schedule install` isn't set up; ignored while it is). Set from the web UI's Settings → Automation card too
