#!/usr/bin/env bash
# release.sh X.Y.Z — tag and publish a release of this nlm fork (see RELEASING.md).
# Refuses unless main is clean and equal to origin/main, CHANGELOG.md has
# "## [X.Y.Z] - DATE", and vet/tests pass. Then: annotated tag vX.Y.Z, push the tag,
# GitHub release "nlm X.Y.Z" with the changelog section as notes and the committed
# binary at the tag as nlm-linux-amd64 (+ .sha256).
set -euo pipefail

die() { echo "release.sh: $*" >&2; exit 1; }
[[ $# -eq 1 && $1 =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "usage: scripts/release.sh X.Y.Z" >&2; exit 2; }
ver=$1 tag=v$1
cd "$(git rev-parse --show-toplevel)"

[[ $(git branch --show-current) == main ]] || die "not on main"
git fetch -q origin
[[ -z $(git status --porcelain) ]] || die "working tree is not clean"
[[ $(git rev-parse HEAD) == $(git rev-parse origin/main) ]] || die "main is not equal to origin/main (push or pull first)"
if git rev-parse -q --verify "refs/tags/$tag" >/dev/null || git ls-remote --exit-code --tags origin "$tag" >/dev/null 2>&1; then
  die "tag $tag already exists"
fi

notes=$(awk -v v="$ver" '
  $0 ~ "^## \\[" v "\\] - " {p=1; next}
  p && /^## \[/ {exit}
  p && /^\[[^]]+\]: / {exit}
  p {print}' CHANGELOG.md | sed -e '/./,$!d')
[[ -n $notes ]] || die "CHANGELOG.md has no '## [$ver] - DATE' section"
grep -q "^## \[$ver\] - [0-9]" CHANGELOG.md || die "the $ver section needs a date"

pkgs=(./cmd/... ./internal/api/... ./internal/batchexecute/...)
go vet "${pkgs[@]}"
go test "${pkgs[@]}"

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
git show "HEAD:nlm" > "$work/nlm-linux-amd64"
(cd "$work" && sha256sum nlm-linux-amd64 > nlm-linux-amd64.sha256)
printf '%s\n' "$notes" > "$work/notes.md"

git tag -a "$tag" -m "nlm $ver"
git push -q origin "$tag"
gh release create "$tag" --title "nlm $ver" --notes-file "$work/notes.md" --verify-tag \
  "$work/nlm-linux-amd64" "$work/nlm-linux-amd64.sha256"
echo "released $tag: $(gh release view "$tag" --json url -q .url)"
