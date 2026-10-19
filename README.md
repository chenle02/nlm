# nlm — NotebookLM from the command line

[![Release](https://img.shields.io/github/v/release/chenle02/nlm?sort=semver)](https://github.com/chenle02/nlm/releases)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go](https://img.shields.io/github/go-mod/go-version/chenle02/nlm)](go.mod)
[![llms.txt](https://img.shields.io/badge/llms.txt-agent%20contract-8A2BE2)](llms.txt)

`nlm` drives [Google NotebookLM](https://notebook.google.com) from a terminal: list
notebooks and sources, create an audio overview, and **download audio and video
overviews as `.m4a` / `.mp4` files** for podcasts and seminar pages.

> **Relation to upstream.** This repository began as a fork of
> [tmc/nlm](https://github.com/tmc/nlm) by Travis Cline, and the credit for the
> original design is his. The two have since **diverged**: upstream rewrote its
> history and most of its code in 2026 (there is no common commit any more), while
> this fork kept its own line and fixed it for NotebookLM's move to
> `notebook.google.com`. We **keep this fork as-is and develop it independently**; it
> is not a staging branch for upstream, and changes are not merged in either
> direction. The decision and the comparison are recorded in [AGENTS.md](AGENTS.md).

## What works

Checked against the live service on 2026-10-18 ([CHANGELOG](CHANGELOG.md), 0.3.0):

| Task | Command |
|---|---|
| List notebooks | `nlm ls` |
| List a notebook's sources | `nlm sources <notebook-id>` |
| List audio/video overviews and their readiness | `nlm artifacts <notebook-id>` |
| Start a new audio overview | `nlm audio-create <notebook-id> "<focus>"` |
| Download the newest audio overview (`.m4a`) | `nlm audio-get <notebook-id> [dir]` |
| Download the newest video overview (`.mp4`) | `nlm video-get <notebook-id> [dir]` |
| PDF → notebook → podcast | `scripts/nlm_workflow.sh paper.pdf` |

Other commands (notes, adding and renaming sources, guides) are carried over from
upstream and were not re-tested after NotebookLM's 2026 changes; video creation is
not implemented (make videos in the web app, then `video-get`). The full command list,
with a tested/untested mark on each, is in [llms.txt](llms.txt).

## Install

```bash
git clone https://github.com/chenle02/nlm
cd nlm
go build -o nlm ./cmd/nlm        # or use the committed Linux binary ./nlm
```

Each [release](https://github.com/chenle02/nlm/releases) also attaches
`nlm-linux-amd64` and its SHA-256.

## Log in

`nlm` uses your NotebookLM browser session: `~/.nlm/env` (cookies and token) and
`~/.nlm/cookies.txt` (all Google cookies, needed for media downloads, which
redirect through accounts.google.com).

- **Automatic refresh.** Google rotates the session cookies, so a saved login lasts
  minutes to hours. When a call is refused as unauthenticated, `nlm` runs
  `nlm-reauth` once if it is on your `PATH` and retries. `nlm-reauth` is a small
  helper that reads the signed-in Firefox profile and rewrites both files (it lives
  in the maintainer's dotfiles; any program that rewrites `~/.nlm/env` and
  `~/.nlm/cookies.txt` works). `NLM_NO_REAUTH=1` turns this off.
- **By hand.** In the browser's DevTools, copy a `batchexecute` request on
  notebook.google.com as cURL and paste it into `scripts/nlm-refresh-auth` (or
  `nlm auth` on stdin).

Never commit or share `~/.nlm/env` or `~/.nlm/cookies.txt`: they are your Google
session.

## Make a podcast from a paper

```bash
nlm audio-create <notebook-id> "the main theorem, for a general math audience"
nlm audio-get <notebook-id> .   # repeat every 30 s until it prints "Saved audio to: ..."
```

Generation usually takes 5–10 minutes; `audio-get` prints `Audio overview is not
ready yet.` until then. `scripts/nlm_workflow.sh paper.pdf` does the whole chain:
create a notebook, upload the PDF, request the overview, poll, and save
`paper.m4a`.

## Configuration

| Variable | Purpose |
|---|---|
| `NLM_COOKIES`, `NLM_AUTH_TOKEN` | the login (normally read from `~/.nlm/env`) |
| `NLM_HOST` | NotebookLM host (default `notebook.google.com`) |
| `NLM_BL` | frontend build label, the page's `cfb2h` value, if Google changes it |
| `NLM_NO_REAUTH` | `1`: do not run `nlm-reauth` on an expired login |
| `NLM_BROWSER_PROFILE` | browser profile for `nlm auth` |

`nlm -debug <command>` prints every request and reply.

## For agents and contributors

- [llms.txt](llms.txt): the agent contract — commands, outputs, login, exit codes,
  limits. A test fails if it misses a command.
- [AGENTS.md](AGENTS.md): how to change the code. [CHANGELOG.md](CHANGELOG.md):
  what changed. [RELEASING.md](RELEASING.md): how a release is cut
  (`scripts/release.sh X.Y.Z`).
- Tests: `go test ./cmd/... ./internal/api/... ./internal/batchexecute/...`
  (`internal/beprotojson` has two failing tests inherited from upstream).

## How it works

NotebookLM has no public API. `nlm` speaks the web app's private RPC
(`batchexecute` under `/_/LabsTailwindUi`): `wXbhsf` lists notebooks, `gArtLc` lists
studio artifacts, `R7cb6c` creates them. Google changes these without notice, so a
sudden failure is most often API drift: check `nlm -debug`, then
[CHANGELOG.md](CHANGELOG.md) for how the last change was handled.

## License

MIT — see [LICENSE](LICENSE). Original work © Travis Cline; fork changes © Le Chen.
