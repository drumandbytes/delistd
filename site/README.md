# site/

The docs site, [delistd.dev](https://delistd.dev/?ref=delistd-site-readme),
built with [Hugo](https://gohugo.io) and deployed to Cloudflare Pages by
`.github/workflows/deploy.yaml` on every push to `main` that touches the broker
data, `site/`, or the guide generator.

## Build locally

```bash
./site/build.sh            # or: ./site/build.sh serve
```

It runs `delistd guides` (writes `site/data/guides.json` and `brokers.json`, which Hugo turns into the directory and the per-broker guide pages in `content/brokers/_content.gotmpl`), `scripts/docs-versions`, copies `data/authorities.yaml` in, and builds into `site/public/`. The generated files are gitignored.

## Layout

- `content/`: hand-written pages (`_index.md`, `guides/*.md`, `authorities.md`);
  the guide pages under `/brokers/` come from `content/brokers/_content.gotmpl`.
- `layouts/`: the theme: base, home, directory, docs and guide templates and partials (`partials/guide-body.md` is the body of a broker guide).
- `static/css/`: `site.css` imports `tokens.css` and `base.css`; the Coral palette mirrors the app's (`internal/web/static/css/tokens.css`).
- `static/CNAME`: the custom domain.

## Versioned docs

`/docs/` has one snapshot per release line plus a nightly one, and a beta one
while a prerelease tag is ahead of the last stable release.
`scripts/docs-versions` builds them (`go run ./scripts/docs-versions`, which
`build.sh` and the deploy workflow both run):

- `next` is the working tree, shown as **Nightly**.
- The newest tag of each stable minor line (`1.2`, `1.1`, ...) is a frozen snapshot of
  that tag's `docs/*.md`. The newest is marked latest and is the only one search
  engines may index; every other version is `noindex`.
- A prerelease tag newer than the last stable release (for example
  `v1.5.0-beta.1`) appears as **1.5 beta** until a stable release passes it.
- 0.x tags are skipped.

Only the user-facing pages listed in `scripts/docs-versions/main.go` are published.
A new release triggers a deploy (`release: published`), which adds its snapshot.
The version menu sits in `layouts/partials/docs-nav.html`; the snapshots and
`data/docs_versions.json` are generated and gitignored. The build needs the release
tags, so run `git fetch --tags` first if a version is missing locally.
