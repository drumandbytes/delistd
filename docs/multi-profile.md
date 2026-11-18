# Multiple profiles

One install can send and track removal requests for more than one identity (a spouse, a family member) from the same config, broker list and inbox, with each profile's history, pipeline status and send jobs kept separate. Most installs need only the profile `delistd init` creates; everything below matters once you run `delistd profile add`.

A single-profile install needs no changes. `config.Load` moves a legacy top-level `profile:` block (pre-0.11) into `profiles:` as `id: default` and keeps the original as `config.yaml.1x.bak`. In the history, a `profile_id TEXT NOT NULL DEFAULT 'default'` column was added to `removal_requests`, `broker_responses` and `pending_tasks`, which back-fills every old row; `TestMigrationBackfillsExistingRowsToDefaultProfile` checks that.

## Config

```yaml
profiles:
  - id: default        # created by `delistd init` or web setup
    first_name: ...
  - id: spouse
    first_name: ...
```

- `GetProfiles()` is the resolved list (one empty `default` before setup); `PrimaryProfile()` is the `default` entry, else the first, and is what `init` edits.
- `GetProfile(id)` resolves one: an empty `id` with a single profile is that profile, an empty `id` with several is an error listing the IDs (this is why `--profile` is "required if ambiguous"), and a non-empty `id` matches case-insensitively or errors.
- `Validate()` checks every profile (unique case-insensitive IDs, required name and email).

Code lives in `internal/config/profiles.go` and `mail.go`.

### Per-profile email accounts

By default every profile shares the top-level `email:` (SMTP) and `inbox:` (IMAP) blocks. A profile can set its own `mail:` block instead, so each person's requests don't pile onto one address, which lowers per-broker send limits and makes replies easier to attribute ([#75](https://github.com/drumandbytes/delistd/issues/75)):

```yaml
profiles:
  - id: spouse
    first_name: ...
    mail:
      email:               # replaces the shared email: block for this profile's sends
        from: spouse@example.org
        smtp: { host: smtp.fastmail.com, port: 465, username: spouse@example.org, password: app-password }
      inbox:               # replaces the shared inbox: block for this profile's replies
        enabled: true
        provider: fastmail # a preset id fills server and port, or set them directly
        email: spouse@example.org
        password: app-password
```

Either field is optional: a profile can send from its own account and still share the default inbox, or the reverse. Send-only providers (Amazon SES) get only `email`, so their replies fall back to the shared inbox.

`EmailForProfile(p)` and `InboxForProfile(p)` return the effective account (the override, else the shared block); every send path goes through `EmailForProfile`. `ConfiguredInboxes()` lists every distinct enabled inbox across profiles, de-duplicated by address: `delistd monitor` scans each (concurrently only under `--watch`), while the web UI's scan, rescan and reclassify cover only the active profile's inbox, so run `monitor` for all of them. `delistd profile add|edit` and the web profile forms (`partials/mail-account.html`, `mailform.go`) set the override with the same presets as setup.

## History scoping

Nearly every `history.Store` method takes a `profileID` and filters by it. Two are deliberately global, `ClearBrokerResponses` and `GetAllBrokerResponses`, because they act on the one shared inbox.

**Shared inbox.** A mailbox shared by several profiles carries all their replies together, so a reply is attributed to the profile that actually emailed that broker, not to whichever profile is active in the session doing the scan. `Store.ResolveProfileForBroker(brokerID)` finds the most recent `removal_requests` row for that broker across all profiles (falling back to `default`), and `inbox.RecordReply` calls it per email before storing the classified reply.

## CLI

The global `--profile <id>` flag goes through `resolveProfile(cfg)` and applies to `send`, `status`, `monitor`, `pipeline`, `fill`, `confirm`, `mark-bounced`, `export`, `draft` and `mark-sent`. `profile list|add|edit|remove` manage the entries. Re-running `init` on an existing config keeps the profiles added with `profile add`, and keeps a `default` entry in sync with the fields you just re-entered.

## Web UI

- The active profile is a plain cookie, `delistd_profile`, not the `SessionStore`, which exists only for the setup wizard. `Server.activeProfile(r)` validates it against the configured profiles and falls back to the first.
- `POST /api/profile` (`handleAPISwitchProfile`) validates `profile_id`, sets the cookie and redirects to a same-origin `redirect` value.
- Every page's template data carries `Profiles`, `ActiveProfile` and `CurrentPath`, so the nav can render the switcher (an auto-submitting `<select>`, shown only with more than one profile) without each handler wiring it.
- Send jobs carry `ProfileID`, and `JobManager.GetActive(profileID)` and `CreateIfNoActive(total, profileID)` are profile-scoped, so two profiles can send at the same time without a "job already active" clash.

## Adding a per-profile data path

A new `history.Store` method or web or CLI path that touches user-specific data takes a `profileID` and filters by it, like the existing ones. If the data comes from the shared inbox, resolve the owner with `ResolveProfileForBroker` rather than using the active profile.
