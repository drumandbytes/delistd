# Delistd

[![CI](https://github.com/drumandbytes/delistd/actions/workflows/ci.yml/badge.svg)](https://github.com/drumandbytes/delistd/actions/workflows/ci.yml)
[![Coverage](https://codecov.io/gh/drumandbytes/delistd/graph/badge.svg)](https://codecov.io/gh/drumandbytes/delistd)
[![Go Reference](https://pkg.go.dev/badge/github.com/drumandbytes/delistd.svg)](https://pkg.go.dev/github.com/drumandbytes/delistd)
[![Release](https://img.shields.io/github/v/release/drumandbytes/delistd)](https://github.com/drumandbytes/delistd/releases)
[![Go Version](https://img.shields.io/github/go-mod/go-version/drumandbytes/delistd)](go.mod)
[![Docker image](https://img.shields.io/badge/ghcr.io-drumandbytes%2Fdelistd-2496ED?logo=docker&logoColor=white)](https://github.com/drumandbytes/delistd/pkgs/container/delistd)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](CONTRIBUTING.md)

**Take back your privacy.** Delistd sends data removal requests to 700+ data brokers on your behalf — for free.

![Delistd dashboard](.github/images/dashboard.png)

Sites like Spokeo, BeenVerified and Whitepages hold your home address, phone number and relatives' names. They're data brokers, and there are hundreds. Services like Incogni and DeleteMe charge $100+/year to send opt-out requests to them; Delistd does the same from your own email account, open source and free.

📖 **[delistd.dev](https://delistd.dev/?ref=delistd-readme)** — broker directory, opt-out guides, and the list of EU/EEA data protection authorities.
🔗 [More Drumandbytes projects](https://drumandbytes.com/projects/?ref=delistd-readme)

<a href="https://alternativeto.net/software/delistd/about/?utm_source=badge&utm_medium=referral" target="_blank"><img src="https://alternativeto.net/static/badges/badge-wide-color.svg" alt="delistd | AlternativeTo" width="320"></a>

**Not technical? Start with the [step-by-step setup guide](docs/getting-started.md)**: downloading Delistd, a Gmail app password, the setup wizard, and a fix for every common error.

> Formerly Eraser. Upgrading from 2.x? The `eraser` command, `ERASER_*` variables and `ghcr.io/drumandbytes/eraser` image are gone in 3.0; your data is unchanged. See [docs/upgrading-to-3.0.md](docs/upgrading-to-3.0.md).

## What to expect

Many brokers act on the email alone. Others reply with a confirmation link, an opt-out form or an identity check. Delistd sends the emails, reads the replies, and shows exactly what needs you (the Pipeline and Tasks pages, or `delistd pipeline`). It is not fully automated, but it does the tedious part: 700+ emails and tracking every reply.

- **Most brokers comply.** GDPR and CCPA oblige them, and many honour requests anyway.
- **It takes time.** Brokers have 30-45 days depending on the law.
- **Repeat it.** Brokers re-list you as they buy and sell data. Delistd re-sends 25 days after a broker's last request (`delistd schedule install` automates it).

## Install

| | |
|---|---|
| **Homebrew** (macOS, Linux with cask support) | `brew tap drumandbytes/tap && brew install --cask drumandbytes/tap/delistd`. If Homebrew 6+ asks you to trust the tap, run `brew trust drumandbytes/tap` and retry. |
| **Prebuilt binary** | Download your OS's file from [Releases](https://github.com/drumandbytes/delistd/releases); the broker list is built in. Windows: the `.exe` or `.zip`. macOS/Linux: `tar xzf` and run `./delistd`. The macOS binary is unsigned: right-click, *Open* once, or `xattr -dr com.apple.quarantine ./delistd`. |
| **Docker** | `docker compose up -d`, then open <http://localhost:8080>; or pull `ghcr.io/drumandbytes/delistd`. Keep the port published to `127.0.0.1`: the web UI has no login. Data, backups, LAN access and scheduling: [docs/docker.md](docs/docker.md). |
| **From source** | With [Go](https://go.dev/dl/): `git clone https://github.com/drumandbytes/delistd.git && cd delistd && go build -o delistd ./cmd/delistd` (on Windows, `-o delistd.exe`). |

`delistd fill` and `delistd confirm` (browser automation for opt-out forms) need Chrome or Chromium; nothing else does.

## Quick start

**Web interface:** run `./delistd serve` (Windows: `.\delistd.exe serve`) and open <http://localhost:8080>. The wizard asks for the details brokers need to find your records and sets up email; then you can browse brokers, send one by one or in bulk, track status, exclude brokers, mark bounces, export an evidence report for a complaint to your data protection authority, and add brokers of your own. Setup takes about ten minutes.

**Command line:**

```bash
./delistd init            # interactive setup
./delistd send --dry-run  # preview; nothing is sent
./delistd send            # send to every broker that is due
./delistd status          # history and stats
```

Everything in the web interface has a command-line twin and the other way round (apart from a few maintainer commands and `confirm`): [docs/ui-cli-parity.md](docs/ui-cli-parity.md).

## Email account

Delistd sends from your own account over SMTP and can read replies over IMAP. Pick your provider in the wizard (or `delistd init`); it fills in the servers. You enter your address and an **app password**, a separate password your provider generates for apps. Never use your main password.

| Provider | What you need | Notes |
|---|---|---|
| Gmail | [App password](https://myaccount.google.com/apppasswords) (needs 2-Step Verification) | ~500 emails/day ([step by step](docs/getting-started.md#step-2-get-an-app-password-gmail)) |
| Proton Mail | [Proton Mail Bridge](https://proton.me/mail/bridge) on the same machine, paid plan | Use the password Bridge shows; its self-signed certificate is accepted because it only listens on localhost |
| Fastmail | App password with IMAP + SMTP | |
| mailbox.org | Application password | |
| Posteo | Your address and password | |
| iCloud Mail | App-specific password | SMTP on port 587 (STARTTLS) |
| Amazon SES | SES SMTP credentials, From address/domain verified in SES | Send-only: monitor replies on a separate mailbox; change the region in the host if yours isn't `eu-west-1` |
| Outlook.com / Hotmail / Live | **Sign in with Microsoft** (OAuth) in setup, or `delistd oauth login` | Microsoft ended password sign-in for personal accounts; see [docs/outlook.md](docs/outlook.md). The approval page calls Delistd's app an "unverified" publisher (verification is pending). Microsoft 365 work accounts aren't covered. |
| Anything else | Choose "Other" and enter the SMTP (and optionally IMAP) server | |

Ports 465 (SMTP) and 993 (IMAP) use TLS from the start; any other port must support STARTTLS. Delistd never sends your password unencrypted. Provider missing or broken? [Open a mail provider issue](https://github.com/drumandbytes/delistd/issues/new?template=mail_provider.yml) or [add it](CONTRIBUTING.md#adding-a-mail-provider).

**Limits.** Delistd caps itself at 450 emails per rolling 24 hours (`options.daily_send_limit`), under Gmail's ~500. Check your provider's cap and lower it if needed. A run resumes where the last stopped, so re-running `delistd send` until nothing is left is safe.

**Automation.** `delistd schedule install` has your OS run Delistd every 6 hours: send to brokers that are due (each re-sent 25 days after its last request, within the daily cap) and check the inbox. `delistd schedule status` shows the last run and `delistd schedule remove` undoes it. Settings → Automation in the web UI does the same, including a "run while this app is open" option for systems without launchd or systemd; `delistd auto` loops in the foreground.

**No credentials at all.** Choose "Skip - I'll send the emails myself" in the wizard (or `options.send_mode: manual`). Delistd then never sends anything: the Brokers page offers each ready-to-send email with an "Open in mail app" button and a **Mark sent** button; on the CLI, `delistd draft <broker-id>` prints one email, `delistd draft -o ./out` writes one `.eml` per broker, and `delistd send --manual` walks the list. `delistd mark-sent <broker-id>` records one so `status`, `pipeline` and `export` still account for it.

## Let a local AI agent handle follow-ups (experimental)

`delistd mcp` connects Delistd to Codex over local STDIO. Ask it to "handle my pending removal tasks", review the proposed batch, and approve it in Delistd (the web UI's **Automation** page, or `delistd mcp approve`). The agent can open confirmation links, fill forms, inspect receipts and save notes and screenshots in the **Tasks** page; email sending uses your settings, templates, limits and history. Codex supplies the AI, so there is no separate API key. CAPTCHAs, logins, identity checks, uncertain submissions and unexpected destinations still need you, and a completed form means the request was accepted, not that data was erased. Chrome or Chromium is required for forms. Experimental: commands and tools may change in a minor release.

Setup, the fake-data demo, the tool list and how approval works: [docs/mcp.md](docs/mcp.md). Tried it? [Tell us how it went](https://github.com/drumandbytes/delistd/issues/new?template=mcp_feedback.yml).

## Command line

| Command | What it does |
|---|---|
| `delistd init` | Interactive config setup |
| `delistd send` | Send removal requests (capped at `daily_send_limit`/day, resumable). `--dry-run` previews, `--resend` ignores the 25-day cooldown, `--ignore-daily-limit` ignores the cap, `--manual` walks the list for hand-sending |
| `delistd draft` / `mark-sent` | Write the emails as `.eml` files to send by hand, then record them as sent |
| `delistd status` / `pipeline` | History and stats; which brokers need follow-up |
| `delistd monitor` | Read your inbox (IMAP) for broker replies; `--reclassify` re-runs the classifier over saved replies, `--watch` keeps running |
| `delistd confirm` / `fill` | Click confirmation links from broker emails; fill opt-out forms with a browser |
| `delistd export` | Evidence report (HTML/JSON) of every request and reply, for a DPA or noyb complaint |
| `delistd list-brokers` | Show the 700+ brokers minus those that only handle other countries than your profile's (`--all-countries` shows all) |
| `delistd add-broker` / `update-brokers` | Add a broker of your own; refresh the list from this repo |
| `delistd mark-bounced` / `cleanup-bounces` | Correct the record for brokers whose email bounced; clear dead addresses (keeps the entry) |
| `delistd audit-brokers` / `validate-brokers` | Check broker sites and domains for signs of life; structural check of a list |
| `delistd schedule` / `auto` | Run sends and inbox checks every 6 hours via the OS, or in the foreground |
| `delistd passphrase set` / `clear` | Add or remove a passphrase on stored mail passwords ([security](#security)) |
| `delistd profile add` / `list` / `edit` / `remove` | Extra profiles, e.g. a household member ([docs/multi-profile.md](docs/multi-profile.md)) |
| `delistd oauth login` | Sign in to Outlook.com |
| `delistd mcp` | Local AI-agent server (experimental) |
| `delistd serve` | The web interface (`-p 3000` for another port) |

Full reference: [docs/commands.md](docs/commands.md).

## Configuration

Your config lives at `~/.delistd/config.yaml`; [config.example.yaml](config.example.yaml) and [docs/commands.md](docs/commands.md#configuration) document every key. The essentials:

```yaml
profiles:
  - id: default
    first_name: Jane
    last_name: Doe
    email: jane@example.com
    country: US          # ISO 3166-1 alpha-2; the wizard and `init` accept a name
    # address, city, state, zip_code, phone, date_of_birth, additional_emails,
    # name_variants, previous_addresses, additional_phones help brokers find you

email:
  from: jane@example.org
  smtp: { host: smtp.fastmail.com, port: 465, username: jane@example.org, password: your-app-password }

options:
  template: gdpr         # gdpr, ccpa or generic
  rate_limit_ms: 2000    # pause between emails
  daily_send_limit: 450  # keep under your provider's cap
  # regions: [us, global]
  # excluded_brokers: [spokeo]
  # excluded_categories: [requires-id]   # brokers that demand an ID document
  # broker_list: verified                # or broker_file: /path/to/your-list.yaml

# inbox:                 # replies, for `delistd monitor`
#   enabled: true
#   provider: fastmail   # or server + port
#   email: jane@example.org
#   password: your-app-password
```

**Templates.** `gdpr` invokes Article 17 (right to erasure) under EU law, `ccpa` the California Consumer Privacy Act, and `generic` references several laws and works anywhere. EU residents should pick `gdpr`: see [EU-NOTES.md](EU-NOTES.md).

### Brokers

The broker database (`data/brokers.yaml`) is compiled into the binary; `delistd update-brokers` pulls a fresh copy into `~/.delistd/brokers.yaml`. Add one with `delistd add-broker` or by hand:

```yaml
- id: example-broker
  name: Example Broker
  email: privacy@example.com
  opt_out_url: https://example.com/optout
  region: us            # us, eu or global
  countries: [DE]       # optional: only people connected to these countries
  category: people-search   # people-search, marketing, background-check, financial-b2b, data-intermediary, device-id-only, requires-id
```

`countries` is for brokers that hold one market's data (a German or Mexican credit bureau) and ask not to be sent requests from elsewhere. They're skipped for profiles whose `country` is outside the list; `send --broker <id>` overrides that.

**A smaller list.** The full list carries some noise (dead addresses, companies that turn out not to be brokers). The *verified* list holds only confirmed data brokers with a current first-party privacy contact, built from registries and privacy notices across the US, EU, Latin America, New Zealand and Kenya (US entries are also checked against California's registry). It does not promise that a broker acts on a request; `status` and `pipeline` show what each one actually does. A weekly CI audit keeps the full list from rotting, clearing dead email domains and opening a PR.

```bash
./delistd send --list verified                   # one run (or options.broker_list: verified)
./delistd send --broker spokeo,pipl              # explicit IDs
./delistd send --region eu --category financial-b2b
./delistd send --exclude broker-id --status never
```

`--broker`, `--region`, `--category` and `--exclude` repeat or take commas. `--status eligible` (the default) skips brokers sent to within 25 days; `never`, `failed` and `all` select by history. The web Brokers page has the same filters.

### The Pipeline

The Pipeline page (or `delistd pipeline`) lists what needs attention. Select a status card to see its requests or actions. **Responses** filters by response type and searches broker names, subjects and saved email bodies; **Action Required** also searches task notes. Filters live in the URL, so refresh and back/forward keep them. Clear "no records for you" replies show as **No data held**.

## Security

- **Your config holds personal data.** Don't commit it. It's created readable only by you.
- **Mail passwords are never stored in plain text.** `smtp.password` and `inbox.password` are saved as `enc:v1:...` (AES-256-GCM, key from this machine's ID and your user). A copied config or synced dotfile can't be decrypted elsewhere, so after moving machines re-enter the passwords (`delistd init` or Settings). Anyone logged in as you can still use Delistd, and so the password, unless you add a passphrase: `delistd passphrase set` makes the passwords undecryptable without it. With one set, unattended runs can't unlock them: `delistd schedule install` is refused, and `delistd auto` and `delistd serve` ask once at startup. There is no recovery; if you forget it, re-enter the passwords.
- **Use app passwords**, and consider a dedicated mailbox to keep removal traffic out of your main inbox.

## How it compares

| | Incogni | DeleteMe | Delistd |
|---|---|---|---|
| Price | $95.88/year (Standard) or $179.88/year (Unlimited), billed annually | $129/year for one person | Free, MIT licence |
| Who sends the requests | Incogni | DeleteMe | You, from your own email account |
| Where your details are kept | With the service | With the service | On your machine only |
| Coverage | 420+ broker sites | Automated plus manual custom removals | 700+ brokers in a published list you can extend |

A paid service saves you time and handles replies and ID checks for you; Delistd gives you control and costs nothing. Prices checked 10 November 2026; the [full comparison](https://delistd.dev/compare/?ref=delistd-readme) has the details, sources and when a paid service is the better choice.

## More

- **EU / GDPR:** read [EU-NOTES.md](EU-NOTES.md) first: which template, safe send volume, and what to do if a broker ignores you.
- **Contributing:** brokers, fixes, templates and docs are welcome: [CONTRIBUTING.md](CONTRIBUTING.md).
- **Code layout and CI:** [docs/architecture.md](docs/architecture.md).

Stuck on setup? [We can help](https://delistd.dev/support/?ref=delistd-readme). Found Delistd useful? A star, a share, or [a few words of feedback](https://github.com/drumandbytes/delistd/issues/new?template=feedback.yml) helps it reach the people who need it.

## License

MIT.

## Disclaimer

Delistd sends legitimate data removal requests based on privacy laws. It isn't legal advice; not every broker must comply with every request, and response times vary.

You are responsible for the accuracy of the data you enter, for your email provider's terms and sending limits, and for anything you submit to a data protection authority. The templates and the `export` report are starting points, not advice on your situation.

## How it was made

Built with the help of an AI coding assistant (Claude). I review and test what gets published.
