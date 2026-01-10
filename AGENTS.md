# Repository Guidelines

## Project Structure & Module Organization
- `cmd/nlm/`: CLI entrypoint and commands (`main.go`, `auth.go`, `env.go`).
- `internal/`: core packages
  - `api/` (NotebookLM client), `batchexecute/` (wrb.fr decoding), `auth/` (browser auth), `rpc/`, `beprotojson/`.
- `proto/` and `gen/`: protobuf definitions and generated Go code.
- `scripts/`: automation (e.g., `scripts/nlm_workflow.sh`, `scripts/nlm_auth_chrome.sh`).
- `docs/`, `examples/`, `specs/`: supplemental docs, samples, and fixtures.

## Build, Test, and Development Commands
- Build local binary: `go build -o nlm ./cmd/nlm`
- Install to GOPATH bin: `go install ./cmd/nlm`
- Run tests: `go test ./...`
- Static checks: `go vet ./...` (add `-v` for verbose)
- Regenerate protos (when editing `proto/`): `buf generate proto`

## Coding Style & Naming Conventions
- Use `go fmt ./...` (or `gofmt -s -w .`) before committing.
- Idiomatic Go: package names are short, lowercase; exported identifiers use CamelCase with doc comments.
- Errors: wrap with `%w`; prefer `errors.Is/As`; return early on failure.
- CLI: add new subcommands/flags under `cmd/nlm/` and keep `flag.Usage` output consistent and concise.

## Testing Guidelines
- Write table-driven tests in `*_test.go`; keep fixtures under `internal/*/testdata/` (see `internal/batchexecute/testdata/`).
- Run `go test -v ./...` locally; add focused tests near changed packages.
- Aim to maintain or increase coverage for touched code paths.

## Commit & Pull Request Guidelines
- Follow Conventional Commits: `feat(scope): …`, `fix(scope): …`, `docs: …`, `refactor: …` (e.g., `feat(nlm): add audio listing`).
- PRs should include: clear description, linked issues, before/after output (e.g., `nlm list`), tests, and docs/README updates when behavior changes.
- Ensure `go fmt`, `go vet`, and `go test ./...` pass before requesting review.

## Security & Configuration Tips
- Do not commit secrets. Auth is stored in `~/.nlm/env`; env vars `NLM_AUTH_TOKEN`, `NLM_COOKIES` are respected.
- Avoid logging sensitive data; guard verbose output behind the `-debug` flag.
- OS-specific auth/browser logic lives under `internal/auth/*`; keep platform gates and fallbacks consistent.

