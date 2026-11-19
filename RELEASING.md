# Releasing CONGA

CONGA is a Go library. A release is a Git tag on the module root plus a GitHub
Release; there are no compiled artifacts to upload.

## Version policy

CONGA stays on `0.x` while its public API is experimental:

- Breaking changes bump the minor version (`0.3.1` -> `0.4.0`).
- New features bump the minor version.
- Fixes and other maintenance bump the patch version.
- `1.0.0` is a deliberate stability commitment, not an automatic bump. At that
  point the version policy changes to standard SemVer.
- `2.0.0` and later require the Go module major-version suffix (`/v2`). The
  module path stays `github.com/conga-sh/conga` for `0.x` and `1.x`.

Do not mark `0.x` releases as GitHub pre-releases. They are normal releases from
an experimental project.

## Commit messages

Releases are generated from [Conventional Commits](https://www.conventionalcommits.org/):

- `feat(scope): ...` adds a user-visible feature or capability and bumps the
  minor version.
- `fix(scope): ...` fixes a defect and bumps the patch version.
- `feat!(scope): ...`, `fix!(scope): ...`, or a `BREAKING CHANGE:` footer marks
  a breaking change and bumps the minor version while the project is pre-1.0.
  Use `feat!` for intentional API redesigns or removals and `fix!` when a defect
  fix changes documented behaviour.
- `docs(scope): ...` appears in the release notes under Documentation; reserve it
  for user-facing reference or guide updates worth announcing, and use
  `chore(docs): ...` for typos, `AGENTS.md`, or internal doc tweaks so they stay
  omitted from public notes.
- `refactor:`, `style:`, `test:`, `ci:`, `build:`, and `chore:` are hidden from
  the release notes and do not trigger a version bump.

Write subjects for CONGA users rather than contributors, and use user-facing
scopes (`cli`, `config`, `completion`, `predict`, `validation`, `errors`,
`features`, `why`, `dotenv`, `maps`, `lint`, `domain-types`, `docs`) instead of
internal package names.

In `BREAKING CHANGE:` footers, keep the message to 1–3 migration-focused
sentences (what broke and the exact action to take) and wrap code symbols,
tags, flags, env vars, error sentinels, and `<placeholder>` tokens in backticks
(for example `` `<PREFIX>_CONFIG` ``, `` `<Name>` ``, `` `env:"-"` ``) so GitHub
does not strip `<...>` as raw HTML.

Prefer squash merges so each pull request lands as one conventional commit on
`main`. Release Please parses the commits since the previous tag to build the
draft release notes and choose the next version.

## Release automation

Release Please maintains a release pull request with the next version, and its
body holds the release notes. The repository keeps no changelog file
(`skip-changelog` in `release-please-config.json`); the GitHub Releases are the
record. Release Please runs after the `check` workflow succeeds on `main`;
merging the release pull request tags the release commit, publishes the GitHub
Release with the notes from the pull request body, and updates the manifest.

Configuration lives in:

- `release-please-config.json`
- `.release-please-manifest.json`
- `.github/workflows/release.yml`

The workflow uses the repository's built-in `GITHUB_TOKEN` with `contents`,
`issues`, and `pull-requests` write access. Enable "Allow GitHub Actions to
create and approve pull requests" in the repository's Actions settings.

Pull requests and tags created with `GITHUB_TOKEN` do not trigger other
workflows. The release pull request therefore has no check runs; merging it
triggers `check` on `main`, and the release is tagged only if that run succeeds.
If branch protection later requires checks on every pull request, add a bypass
for the release pull request or switch to a GitHub App token.

The tag guard in `.github/workflows/release-guard.yml` rejects manually pushed
tags outside `v0.MINOR.PATCH` and tags whose commit is not reachable from
`main`. Tags created by Release Please use `GITHUB_TOKEN` and do not trigger
that workflow. Repository tag rulesets should prevent tag updates and deletion;
allow GitHub Actions to create tags when tag creation is restricted.

## Cutting a release

1. Merge normal pull requests into `main`.
2. Wait for Release Please to open or update the release pull request.
3. Confirm the proposed version matches the version policy.
4. Curate the release notes in the pull request body right before merging (every
   subsequent merge to `main` regenerates the body and drops hand edits):
   - Add a 2–3 sentence summary and a **Highlights** section at the top.
   - Group **Breaking Changes & Migration** by impact (API changes with brief
     before/after Go snippets, schema/validation changes, behaviour/diagnostic
     changes), and verify all symbols and `<placeholder>` tokens are backticked.
   - Deduplicate entries that already appear under Breaking Changes from Features
     or Bug Fixes, remove internal-only noise and redundant commit SHA links, and
     end with the compare link, minimum Go version, and docs/`pkg.go.dev` links.
5. Merge the release pull request.
6. Wait for the `check` workflow on `main` to pass; Release Please then tags and
   publishes the release.
7. Confirm the tag and the GitHub Release notes.

The release pull request is the review point. Do not merge it merely to clear
the queue.

### Release notes template

```markdown
Short 2–3 sentence summary of what changed in this release and who should upgrade.

### 🚀 Highlights

- **Headline:** concise user-facing takeaway (#PR).

### ⚠ Breaking Changes & Migration

#### API changes
- **`RemovedOrChangedSymbol`** (#PR): what changed and before/after Go snippet if helpful.

#### Schema & validation changes
- **Summary** (#PR): what fails at startup now and how to update structs or tags.

#### Behaviour & diagnostics
- **Summary** (#PR): runtime or error-message change and migration note.

### ✨ Features

- **scope:** user-facing description (#PR)

### 🐛 Bug Fixes

- **scope:** user-facing description (#PR)

### 📚 Documentation

- **scope:** user-facing description (#PR)

---

**Full Changelog:** https://github.com/conga-sh/conga/compare/vPREV...vNEXT
**Requirements:** Go 1.27+ · [Docs](https://docs.conga.sh) · [pkg.go.dev](https://pkg.go.dev/github.com/conga-sh/conga)
```

### First release

The first tag (`v0.1.0`) is cut manually from prepared release notes before
Release Please is enabled. Push the first-release commit to `main`, tag it, and
publish the release; add `release.yml` afterward so the first Release Please run
sees the existing tag.

## After publishing

- Confirm the module resolves:

  ```sh
  go list -m github.com/conga-sh/conga@v0.x.y
  ```

- Confirm the tag renders on pkg.go.dev.
- Never move, force-push, or delete a published tag. The Go checksum database
  caches module content by version; retagging breaks consumers. Publish a patch
  release instead.
