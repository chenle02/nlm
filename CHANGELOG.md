# Changelog

All notable changes to this fork of `nlm` are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versions follow
[Semantic Versioning](https://semver.org/). While 0.x, a minor bump marks a new
command or a new user-visible flag or behavior; a patch carries only fixes and
docs. The policy and the steps are in [RELEASING.md](RELEASING.md).

This fork has diverged from [tmc/nlm](https://github.com/tmc/nlm) and is developed
on its own line; see "Relation to upstream" in [README.md](README.md).

## [Unreleased]

## [0.3.0] - 2026-10-18

### Added

- `nlm artifacts <notebook-id>`: lists a notebook's studio outputs (audio and
  video overviews and others) with kind, readiness, length and title, from the
  `gArtLc` RPC the web app uses.
- `nlm video-get <notebook-id> [dir]`: downloads the newest ready video overview
  as `.mp4`. Video download did not exist before.
- Media downloads use `~/.nlm/cookies.txt` (Netscape format), because the media
  links redirect through accounts.google.com and need that host's cookies.
- When the login has expired, `nlm` runs `nlm-reauth` once (if it is on `PATH`)
  and retries; `NLM_NO_REAUTH=1` turns this off.
- `NLM_HOST` and `NLM_BL` override the NotebookLM host and the frontend build
  label without a rebuild.
- `llms.txt`: the agent contract for this fork; a test checks that it covers
  every command.

### Changed

- NotebookLM moved to `notebook.google.com`; every request and the browser-login
  paths use it, with the current build label.
- `nlm audio-get <notebook-id> [dir]` now downloads the newest audio overview as
  `.m4a` (the web app's Download link: about 6 MB/s, where the streaming link is
  throttled to about 32 KB/s). The old `audio-get <id> <type>` form still works.
- `nlm audio-create` creates the overview with `R7cb6c`, the create-artifact RPC
  the web app uses, over all the notebook's sources, and prints the new id.
- `audio-get`/`video-get` consider only the newest artifact of the kind, so right
  after `audio-create` they report "not ready" instead of downloading an older
  one; a 404 on a just-finished overview also counts as not ready.
- `scripts/nlm_workflow.sh` saves `.m4a`.
- `video-create` and `video-rm` say they are not supported yet instead of sending
  placeholder RPC ids.

### Fixed

- An expired login made `nlm ls` print an empty list. Google's chunk lengths do
  not match the bytes; the decoder now falls back to one JSON chunk per line and
  ignores the closing `["e",...]` chunk, so the expiry is reported.
- `audio-create` silently returned the existing overview: `AHyHrd` no longer
  creates one.
- The usage text advertised seven commands that did not run: `notes`, `edit-note`
  and `refresh-source` are now dispatched (their code existed); `check-source`,
  `analytics`, `share` and `feedback` are removed from it (commented out in the code).
  A test now fails if the usage lists a command that is not dispatched.
- The committed binary was stale (it sent a 2024 build label); it is rebuilt.
- The root `nlm_workflow.sh` symlink pointed at an absolute home path; it is now
  relative to `scripts/nlm_workflow.sh`.

## [0.2.0] - 2026-01-18

### Fixed

- Detect Google API error code 16 (UNAUTHENTICATED) and trigger re-authentication.
- Synced the `bl` (build label) and `f.sid` parameters with NotebookLM.
- URL-encoded auth tokens from pasted cURL commands are decoded.

### Added

- Manual cURL authentication through browser DevTools (`nlm auth` on stdin,
  `scripts/nlm-refresh-auth`), and a warning when captured cookies lack
  SID/SSID.

## [0.1.0] - untagged

### Fixed

- Multi-chunk response decoding in `nlm list`, and an infinite recursion in the
  batchexecute response parser.
- Authentication timeout handling.

### Added

- A framework for video overview commands (`video-create`, `video-get`,
  `video-rm`), without working RPC ids.

[Unreleased]: https://github.com/chenle02/nlm/compare/v0.3.0...HEAD
[0.3.0]: https://github.com/chenle02/nlm/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/chenle02/nlm/releases/tag/v0.2.0
