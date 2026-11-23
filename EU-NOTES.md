# GDPR (EU / EEA) notes

Delistd suits an EU/EEA resident exercising GDPR rights (Article 17, erasure); CCPA requests are supported too. It defaults to the `gdpr` template, and this file covers the EU-specific details.

## Setup

1. Install Delistd ([README](README.md#install)).
2. `delistd init`, and when it asks for a template pick **gdpr** (not `ccpa` or `generic`): Article 17 is the right legal basis to cite.
3. `delistd send --dry-run` to preview, then `delistd send`.

## Brokers

The list is mostly US-region on purpose: US platforms, ad-tech and breach-sourced people-search sites hold EU residents' data too. EU and UK entries with a direct opt-out address are added as they are found (`delistd list-brokers --region eu` shows them), among them credit bureaus (SCHUFA, Creditreform, CRIF, Creditsafe, Dun & Bradstreet), address-marketing data (Deutsche Post Direkt) and ad-tech (Adform, Criteo, RTB House, Teads, Equativ). The exact count moves as entries are added and dead ones removed, so the docs quote a round floor ("1,000+"); `grep -c '^    - id:' data/brokers.yaml` gives the true one. Most EU entries are also in the smaller `data/brokers-verified.yaml` (`send --list verified`; see [architecture.md](docs/architecture.md)).

**Country-scoped brokers.** About a hundred entries carry `countries:` (`schufa`: `[DE]`, `regis24`: `[DE, AT]`, `192-com`: `[GB]`, plus credit bureaus and directories in Latin America, Africa, Asia, Oceania and across Europe). The list covers people worldwide, but most of these brokers hold one market's data, and requests from elsewhere only produce no-match replies, so Delistd skips them for profiles whose `country` is outside the list. Most US brokers are deliberately not scoped. If you lived in a scoped country before, name the broker: `send --broker <id>`.

**Brokers without an email.** Many brokers, in the US and the EU alike, don't take a removal email: they want an Article 15/17 request through their own form or DSR portal (often Termly, OneTrust or saymine.io). About a fifth of the list is already like that (an `opt_out_url` and no `email`): `send` skips them and prints the form link, and the brokers page shows them as needing action. An EU or UK broker with a real DSR page belongs in `brokers.yaml` the same way, with `countries:` when it states it holds one market's data. Delistd doesn't auto-fill portals: that isn't safe to automate.

**Not in `brokers.yaml`.** Ad-tech whose deletions are cookie-identifier based (they don't apply if you aren't carrying their cookies), brokers with no stable DSR page or address, and EU Data Governance Act "data intermediation services" (data wallets, personal data spaces), which hold your profile only if you signed up. Track those yourself (a spreadsheet works) rather than in this repo.

## If a company ignores or refuses a request

Controllers have one month to respond, extendable to three for complex requests. If they miss that or refuse without a valid legal basis, you can complain (Article 77) to the supervisory authority of the EU/EEA country where you live, work, or where the infringement happened. The authorities and their websites are in [data/authorities.yaml](data/authorities.yaml); `delistd export` also names the one for your profile's `country` (an ISO code, e.g. `LV`).

Expect DPAs to be slow: noyb.eu shows some of its own complaints pending for years. In practice the request itself, and the liability it creates, is usually what gets compliance. [noyb.eu](https://noyb.eu) has complaint templates and occasionally takes cases.

`delistd export` builds the evidence to attach: per broker, what was sent and when, the reconstructed request, every reply with its date, and the controllers past the one-month deadline with no substantive answer. `--format html` opens in a browser and prints to PDF.

- **Germany:** a complaint against a private company normally goes to the supervisory authority of your own federal state (Land), not the federal BfDI; the [BfDI site](https://www.bfdi.bund.de) links all 16.
- **UK:** outside GDPR and the EDPB since Brexit, but the UK GDPR mirrors the right to erasure. Complain to the ICO: <https://ico.org.uk/make-a-complaint/>.

## Sending by hand

A request sent from your own mail client is exactly as valid as one Delistd sends. To avoid configuring an app password, set `options.send_mode: manual` (or choose "Skip" in the web setup): `delistd draft` and `delistd send --manual` render the emails, you send them, and `delistd mark-sent` records them so `export` and `pipeline` still work.

## Sending 1,000+ emails safely

`send` caps itself at `options.daily_send_limit` (450 by default, under Gmail's ~500/day; lower it if your provider caps lower) per rolling 24 hours, and skips any broker it already emailed successfully in the last 25 days. So just run `delistd send` again, the same day or the next, until it reports nothing left, or let `delistd schedule install` run it every 6 hours. It resumes where it stopped without double-emailing anyone. `--ignore-daily-limit` sends everything in one run (only if your provider can take that volume); `--resend` re-sends even inside the 25-day cooldown.

## Recurring maintenance

Brokers re-scrape and re-list you continuously. Re-run `delistd send` and revisit unresolved manual items (portal and DSR brokers, form-required replies) every 60-90 days.
