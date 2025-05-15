#!/usr/bin/env bash
# Determine the directory of this script
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Automate NotebookLM workflow with nlm CLI
# Usage: ./nlm_workflow.sh <document.pdf>
# Requirements: nlm CLI configured & authenticated

set -euo pipefail
IFS=$'\n\t'

log() {
  local msg="$1"
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] $msg" >&2
}

usage() {
  cat <<EOF
Usage: $0 <path-to-pdf>
Example: $0 chen_dalang-15-moments.pdf
Automates:
  - notebook creation (if needed)
  - PDF upload
  - audio generation
  - audio download
EOF
  exit 1
}

check_dependencies() {
  for cmd in nlm grep sed head mktemp; do
    if ! command -v "$cmd" &>/dev/null; then
      echo "Error: '$cmd' is required but not installed." >&2
      exit 1
    fi
  done
}

check_notebook_exists() {
  local title="$1"
  log "Checking if notebook '$title' exists..."
  # List notebooks and look for the title substring
  if nlm list 2>/dev/null | grep -Fq "$title"; then
    return 0
  fi
  return 1
}

create_notebook() {
  local title="$1" out id
  log "Creating notebook '$title'..."
  # Create notebook and capture output to extract ID
  out=$(nlm create "$title" 2>&1) || {
    log "Failed to create notebook: $out"
    exit 1
  }
  # Extract notebook ID (UUID)
  id=$(echo "$out" | grep -Eo '[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}' | head -1)
  if [[ -z "$id" ]]; then
    log "Failed to parse notebook ID from create output"
    exit 1
  fi
  echo "$id"
}

upload_pdf() {
  local id="$1" pdf="$2"
  log "Uploading '$pdf' to notebook id '$id'..."
  # Add PDF as source to notebook
  if ! nlm add "$id" "$pdf"; then
    log "Upload failed"
    exit 1
  fi
}

generate_and_download_audio() {
  local id="$1" output="$2" interval=10 tmp
  log "Requesting audio overview for notebook id '$id'..."
  if ! nlm audio-create "$id" "Generate audio overview"; then
    log "Failed to request audio overview"
    exit 1
  fi
  log "Polling for audio readiness..."
  while true; do
    tmp=$(mktemp)
    nlm audio-get "$id" >"$tmp" 2>/dev/null || true
    if head -c 4 "$tmp" | grep -q 'RIFF'; then
      log "Audio ready; saving to '$output'"
      mv "$tmp" "$output"
      break
    else
      log "Audio not ready; retrying in ${interval}s"
      rm -f "$tmp"
      sleep "$interval"
    fi
  done
}

main() {
  [[ $# -eq 1 ]] || usage
  local pdf="$1"
  [[ -f "$pdf" ]] || { echo "Error: '$pdf' not found." >&2; exit 1; }

  check_dependencies

  local base
  base=$(basename "${pdf%.*}")

  if check_notebook_exists "$base"; then
    log "Notebook '$base' already exists. Exiting."
    exit 0
  fi

  # Create notebook and get its ID
  nb_id=$(create_notebook "$base")
  upload_pdf "$nb_id" "$pdf"

  # Generate and download audio overview (.wav)
  local outfile="${script_dir}/${base}.wav"
  generate_and_download_audio "$nb_id" "$outfile"

  log "Workflow complete. Audio saved to '$outfile'"
}

main "$@"