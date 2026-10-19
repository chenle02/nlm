# Releasing nlm (this fork)

A release is a `CHANGELOG.md` section, a commit that carries it, one annotated
tag `vX.Y.Z`, and a GitHub release whose notes are that section and whose asset
is the committed Linux binary. `scripts/release.sh X.Y.Z` does the tag and the
GitHub release; the changelog and the binary are yours to prepare.

## When to tag

While the fork is 0.x:

| `## [Unreleased]` holds | Version |
|---|---|
| Any `### Added` entry: a new command, or a new user-visible flag, env var or behavior | minor: `0.(N+1).0` |
| A `### Changed` entry that changes a command's output, files or exit codes | minor: `0.(N+1).0` |
| Only `### Fixed` entries and documentation | patch: `0.N.(k+1)` |
| Nothing user-visible | no tag; it rides with the next release |

Command names are kept: a renamed command stays as an alias with the same output.

## Steps

1. Move the `## [Unreleased]` entries into `## [X.Y.Z] - YYYY-MM-DD`, leave an
   empty `## [Unreleased]`, and update the compare links at the bottom.
2. If a command was added or renamed, update the usage text in `cmd/nlm/main.go`,
   `README.md` and `llms.txt` (the test `TestLLMSTxtCoversEveryCommand` fails
   until `llms.txt` names every command).
3. Rebuild the committed binary: `go build -o nlm ./cmd/nlm`.
4. Run the checks: `go vet ./cmd/... ./internal/api/... ./internal/batchexecute/...`
   and `go test ./cmd/... ./internal/api/... ./internal/batchexecute/...`.
   (`internal/beprotojson` has two failing tests inherited from upstream; they
   predate this fork's changes.)
5. Commit `release: X.Y.Z` with the changelog, the binary and any docs, and push
   `main`.
6. `scripts/release.sh X.Y.Z`: refuses unless `main` is clean and equal to
   `origin/main`, the changelog has the section, and the checks pass; then tags,
   pushes the tag and creates the GitHub release with `nlm-linux-amd64` and its
   `.sha256`.
7. Pull on the other machines (`git pull` in `~/Dropbox/Public/nlm`).
