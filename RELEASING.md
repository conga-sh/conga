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

- `feat: ...` adds a user-visible feature and bumps the minor version.
- `fix: ...` fixes a defect and bumps the patch version.
- `feat!: ...` or a `BREAKING CHANGE:` footer marks a breaking change and bumps
  the minor version while the project is pre-1.0.
- `docs:`, `test:`, `ci:`, `build:`, and `chore:` do not create a release.

Prefer squash merges so each pull request lands as one conventional commit on
`main`. Release Please parses the commits since the previous tag to build the
changelog and choose the next version.

## Release automation

Release Please maintains a release pull request with the next version and
`CHANGELOG.md` update. It runs after the `check` workflow succeeds on `main`;
merging the release pull request tags the release commit, publishes the GitHub
Release, and updates the manifest.

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
3. Confirm the proposed version matches the version policy and the changelog is
   accurate.
4. Merge the release pull request.
5. Wait for the `check` workflow on `main` to pass; Release Please then tags and
   publishes the release.
6. Confirm the tag, GitHub Release, and changelog update.

The release pull request is the review point. Do not merge it merely to clear
the queue.

### First release

The first tag (`v0.1.0`) is cut manually from the prepared changelog before
Release Please is enabled. Push the first-release commit to `main`, tag it, and
publish the release; add `release.yml` afterward so the first Release Please run
sees the existing tag.

## After publishing

- Confirm the module resolves:

  ```sh
  go list -m github.com/conga-sh/conga@v0.1.0
  ```

- Confirm the tag renders on pkg.go.dev.
- Never move, force-push, or delete a published tag. The Go checksum database
  caches module content by version; retagging breaks consumers. Publish a patch
  release instead.
