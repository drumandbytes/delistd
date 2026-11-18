# Upgrading to 3.0

3.0 removes the old Eraser names that [stability.md](stability.md) kept working since the rename in 2.3. If you already use the `delistd` command, the `DELISTD_*` variables and the `ghcr.io/drumandbytes/delistd` image, there is nothing to do. Your data is untouched: a `~/.eraser` directory is still found, and your encrypted mail passwords keep working.

## What was removed

| Removed | Use instead | What to do |
|---|---|---|
| The `eraser` command (the alias shipped beside `delistd`) | `delistd` | Change scripts, shell aliases and cron entries. The OS schedule: see below |
| `ERASER_*` environment variables (`ERASER_CONTAINER`, `ERASER_MACHINE_ID_FILE`, `ERASER_ALLOWED_HOSTS`, ...) | The same name with `DELISTD_` | Rename them in compose files, systemd units and Kubernetes manifests. They are no longer read at all, so a leftover `ERASER_*` setting silently does nothing |
| The `ghcr.io/drumandbytes/eraser` image tag | `ghcr.io/drumandbytes/delistd` | Change `image:` in compose and Kubernetes. The old tag stays at its last release (2.x) and gets no more updates |
| The rename notices (the "Eraser is now Delistd" card and the CLI line) | none | Nothing |
| The automatic move of a pre-rename OS schedule from `eraser` to `delistd` | `delistd schedule install` | See below |

## The OS schedule

An OS schedule (launchd on macOS, systemd on Linux) installed before 2.3 runs the `eraser` command. Releases 2.4 to 2.x moved it over to `delistd` the first time any command ran. 3.0 no longer does that, and the `eraser` binary is gone, so a schedule that was never migrated stops working. If you went straight from a release before 2.4 to 3.0, run this once:

```bash
delistd schedule install
```

It rewrites the schedule to run `delistd`. Check it with `delistd schedule status`.

## Docker

The image sets `DELISTD_CONTAINER` and `DELISTD_MACHINE_ID_FILE` itself, and keeps its data in `/home/nonroot/.delistd`. If your compose file or manifest overrode the old `ERASER_*` names, rename them. The compose file and the Kubernetes manifest now name the volume `delistd-data` and mount it at `/home/nonroot/.delistd`. To keep your data, either keep your old volume's name and change only the mount path (the machine ID that decrypts your stored passwords lives inside it), or copy it across once:

```bash
docker run --rm -v eraser-data:/from -v delistd-data:/to debian:trixie-slim cp -a /from/. /to/
```

## Homebrew

`brew upgrade` installs only `delistd` now. The old `eraser` binary is removed from the cask.

## Renamed data and schedule names

- **Data directory:** `~/.delistd` for new setups. A `~/.eraser` that holds your `config.yaml` or `history.db` is still found and used; nothing is moved (`mv ~/.eraser ~/.delistd` if you like).
- **OS schedule:** the launchd label and systemd unit are now `com.drumandbytes.delistd.auto` and `delistd-auto`. `delistd schedule install` replaces a job under the old names; so does `schedule remove`.
- **Mail archive folder:** the default is `Delistd`. If you rely on the old default folder, set `inbox.archive_folder` to its name in your config to keep scanning it.
- **Stored passwords** keep working: they are re-encrypted under the new key the next time the config is saved.
- **Web UI:** the session cookies are renamed, so open tabs sign you out once.

The `eraser.drumandbytes.dev` redirect to delistd.dev stays.

## Other changes in 3.0

Much of the internals was rewritten (the reply classifier and link parser, the web core and templates, the history store, the browser automation and the IMAP monitor), each against a regression corpus recorded from the old code, so behaviour is the same except where noted:

- **History times are stored in UTC.** A one-time conversion runs the first time `history.db` is opened. Downgrading afterwards isn't supported (see [stability.md](stability.md)).
- **Reply classifier ties** (equal scores between two verdicts, about 9% of mixed replies) used to depend on map order; they are now settled the same way every time and sent to review.
- **Mail in ISO-8859-x and windows-125x** is decoded (those bodies used to arrive empty). Outgoing mail has a `Date` header, RFC 2047 subjects and quoted-printable bodies for non-ASCII text.
- **Archiving without IMAP MOVE** removes only the messages it archived when the server supports UIDPLUS; without it, other mail you flagged `\Deleted` can still go with them and Delistd warns.
- **`config.yaml` and the broker lists are written atomically**, so a crash or a full disk can't truncate them.
- **The Brokers page** lists 50 brokers at a time with "Show more" (about 200 KB instead of 2 MB).
