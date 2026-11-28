# Auditing

The codebase gets periodic dead-code and security sweeps. To repeat one:

```bash
go build ./... && go vet ./... && go test -race ./...
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest run --max-issues-per-linter=0 --max-same-issues=0 ./...
go run honnef.co/go/tools/cmd/staticcheck@latest ./...
go run golang.org/x/tools/cmd/deadcode@latest ./...      # unreachable-from-main functions
go run golang.org/x/vuln/cmd/govulncheck@latest ./...    # known CVEs in reachable code paths
```

`govulncheck` also runs in CI on every PR and weekly (`.github/workflows/vulncheck.yml`), so new advisories show up without a code change.

`golangci-lint` (config: `.golangci.yml`: errcheck, govet, staticcheck, unused) also runs in CI, so a clean `go build`, `go vet` and `go test` isn't the whole picture. Its default caps hide duplicates past the first few and read as "mostly clean", so pass `--max-issues-per-linter=0 --max-same-issues=0` when you audit.

## Country scope

When a broker's own reply, privacy policy or registry entry says it holds data for one country only, add `countries: [XX]` (upper-case ISO code) to that entry, and note where that was stated. Don't scope on a guess from the domain or HQ: a wrong scope hides the broker from people it does hold data on.

## Growing the broker list from state registries

The US state data-broker registries are public and worth mining for entries the
list is missing. Each is a one-click download, not an API:

- **California** (CPPA, DELETE Act): https://cppa.ca.gov/data_broker_registry; the largest, 600+ brokers. "Download Registration Information" is the current year's CSV (`registry.csv`); earlier years are `registry2025.csv`, `registry2024.csv` and `complete-reg-data-brokers.csv` (2020-2023). Scripted downloads time out, so use a browser. Columns: `Data broker name:`, `Data broker primary website:`, `Data broker primary contact email address:`
- **Oregon** (Division of Financial Regulation): https://dfr.oregon.gov/business/licensing/data-broker-registry; search at `ordcbs.mylicense.com` (profession "DFR-Data Broker", license type "Data Broker", name blank), which lists each broker's opt-out email, website and instructions. Columns of the export: `FACILITY NAME`, `OPT OUT EMAIL`, `OPT OUT WEBSITE`. Check the export parses as CSV (a stray quote can break it)
- **Texas** (Secretary of State): https://www.sos.texas.gov/statdoc/data-brokers.shtml; searchable, no bulk export found
- **Vermont** (Secretary of State): https://sos.vermont.gov/data-brokers/; the page moved and 404s; find the current registry from the Secretary of State's site
- Connecticut (from 2027) and New Jersey (registry not operative until 2027) will add two more.

Outside the US there is no public data-broker register to mine. Quebec's CAI publishes
a register of "personal information agents" (credit bureaus and investigators), and the EDPB
published a data-brokers market study in 2026 (a study, not a register).

Registry rows carry typos (a dead email domain, a website spelled wrong, a URL with a trailing
`.` or `'` for `?`), several legal entities of one group share a mailbox, and some are brokers
we list under another id. Before adding rows: fetch each website and opt-out URL, compare the
email domain with the website's, and keep one entry per mailbox.

Download a registry as CSV, then:

```bash
go run ./scripts/import-registries -csv registry.csv \
    -name-col "Data Broker Name" -url-col "Website" -email-col "Email Address"
```

It diffs the rows against the embedded list and writes `candidates.yaml` (likely
new, in `brokers.yaml` shape) and `review.md` (fuzzy matches to eyeball). Fill in
`category` and a working `email`/`opt_out_url` for the candidates you keep, move
them into `data/brokers.yaml`, then run `delistd audit-brokers` to drop any that
are already dead. Both output files are gitignored.

### The verified list (`data/brokers-verified.yaml`)

A second, smaller embedded list offered as `delistd send --list verified` /
`options.broker_list: verified`. The bar for an entry: a company we can
confidently call a data broker (US: cross-checkable against a state registry;
elsewhere: a regulator-recognized credit bureau or established data-intelligence
company) **and** a current first-party privacy email or rights portal. It's a
curated subset, not a mirror of `brokers.yaml`; the point is
signal, not coverage.

Grow it the same way as the main list, but keep only entries you can stand
behind:

```bash
# US: diff a state registry CSV against BOTH lists, fill category + contact,
# add the good ones to data/brokers-verified.yaml (and usually brokers.yaml too)
go run ./scripts/import-registries -csv ca-registry.csv -name-col "Data Broker Name" \
    -url-col "Website" -email-col "Email Address"

# EU: credit bureaus + GVL big names with a direct privacy address
go run ./scripts/import-registries -gvl https://vendor-list.consensu.org/v3/vendor-list.json
```

`delistd validate-brokers` (no arg) checks this list too, with a lower entry-count
floor (`MinVerifiedBrokerCount`).

### The EU/EEA side

There is no EU equivalent of the state registries, GDPR replaced the old
national notification registers with internal Article 30 records, which aren't
public. So EU expansion is per-country research. The sources worth mining:

- **IAB TCF Global Vendor List**: `https://vendor-list.consensu.org/v3/vendor-list.json`,
  ~1000 ad-tech vendors active in the EU market, versioned and current. The
  importer reads it directly:

  ```bash
  go run ./scripts/import-registries -gvl https://vendor-list.consensu.org/v3/vendor-list.json
  ```

  Caveat: these are mostly cookie/RTB-based, so a name-based erasure email
  often matches nothing ("we don't hold identifiable data on you" - the same
  point EU-NOTES.md makes). Even filtered to vendors that self-declare holding
  profiles / user-provided data, it produces ~380 candidates with no opt-out
  email, each needing DSR-portal research. Treat it as a **cross-check for
  missing big names** (LiveRamp, Epsilon, Adform, Audience Solutions, ...), not
  a bulk import. `-gvl-all` drops the data-category filter.
- **Credit reference & B2B-intelligence bureaus**: the EU entities that
  actually hold name-linked profiles and must action Article 17: SCHUFA,
  Creditreform, Regis24, CRIF, Dun & Bradstreet / Bisnode, Creditsafe,
  arvato/Infoscore, Acxiom, Experian and Equifax EU arms, Schober, Deutsche
  Post Direkt, AZ Direct. Most take requests only through a form or DSR portal
  (so they're tracked outside `brokers.yaml` per EU-NOTES.md), but the ones
  with a direct privacy address belong in the list.
- **YourOnlineChoices / EDAA** (youronlinechoices.eu): the European ad
  preference programme; participating companies have a defined opt-out.
- **noyb** and **GDPRhub** case pages, name specific controllers.

## Keeping `data/brokers.yaml` honest

Two checks, both also wired into CI:

- **Structure**: `delistd validate-brokers` (or `go test ./internal/broker/...`,
  which runs `Validate` against the embedded copy on every PR): a sane entry
  count, required `id`/`name`, unique ids, known `region` values, plausible
  emails, well-formed URLs. Run `delistd validate-brokers candidates.yaml` before
  merging a batch from the importer.
- **Liveness**: `.github/workflows/broker-audit.yml` runs `delistd audit-brokers
  --fix` every Monday. `--fix` clears the email of any broker whose mail domain is
  dead (MX and host lookup both fail), keeping the row, blanking the address, and
  appending a dated note. The workflow then opens a PR against `data/brokers.yaml`
  if anything changed. A quiet week opens no PR. Review the PR before merging;
  investigate `website-dead` / `unknown` entries (printed in the run log, never
  auto-edited) by hand. A "No working channel left" section lists entries whose
  email is dead *and* whose website is unreachable with no `opt_out_url`, remove
  those by hand rather than keeping a row nobody can contact. Also runnable from
  the Actions tab on demand. Run `delistd audit-brokers --fix` locally with
  `--brokers <path>` to do the same to a working copy.
- **Canonical formatting**: `--fix` (and the bounce tooling) save with
  `yaml.Marshal`, so a hand-edited list that isn't already in that form is
  reformatted wholesale on the next audit PR. `TestEmbeddedListsCanonical` fails
  until it is; `go test ./internal/broker -run TestEmbeddedListsCanonical -update`
  rewrites `data/brokers*.yaml` in place.
- **Dated evidence**: when you add or change a broker's email or opt-out URL, record the date you verified it and the source you checked (the broker's privacy page or registry entry). A channel with no dated verification is unconfirmed, even if it passes the checks above; the verified list needs this for every entry.

## Dead code

`deadcode` and `staticcheck` report false positives for anything reached only through `text/template` reflection (a template calling `{{.SomeMethod}}`). Grep the `.html` templates for `{{[^}]*SomeSymbol[^}]*}}` before deleting what they flag, not a plain substring ("Country" contains "Count"). Before removing something, confirm it has no caller with `deadcode` plus a grep; what was removed and why is in the git history, so look there before rewriting a helper from scratch.

Two tests catch drift for you: `go test ./internal/web -run 'TestStylesheetClasses|TestTemplateUtility'` flags CSS rules and template classes that no longer match, and `scripts/stylesnap` shows what a CSS or template change did to every page ([architecture.md](architecture.md#web-ui-static-assets)).
