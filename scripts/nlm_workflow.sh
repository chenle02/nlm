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
  
  # Get notebook list and clean up the output
  local notebooks
  notebooks=$(nlm list 2>/dev/null | tail -n +3 | sed -E 's/[[:space:]]*$//' | sed -E 's/^[[:space:]]*//')
  
  # Debug output
  log "Available notebooks:"
  echo "$notebooks" | while read -r line; do
    log "  '$line'"
  done
  
  # Check for exact title match (ignoring leading/trailing spaces)
  if echo "$notebooks" | grep -q "^$title$"; then
    log "Found exact match for notebook '$title'"
    return 0
  fi
  
  # Check for title as a word boundary (to avoid partial matches)
  if echo "$notebooks" | grep -q "\b$title\b"; then
    log "Found notebook containing '$title' as a complete word"
    return 0
  fi
  
  log "No matching notebook found for '$title'"
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
  
  # First verify the notebook exists
  if ! nlm sources "$id" &>/dev/null; then
    log "Error: Notebook ID '$id' not found or inaccessible"
    exit 1
  fi
  
  # Try to upload with timeout
  local max_attempts=3
  local attempt=1
  local success=false
  
  while [[ $attempt -le $max_attempts ]]; do
    log "Upload attempt $attempt of $max_attempts..."
    
    # Run nlm add and capture both stdout and stderr
    local out
    out=$(nlm add "$id" "$pdf" 2>&1)
    local exit_code=$?
    
    # Check for various success indicators
    if [[ $exit_code -eq 0 ]] || \
       echo "$out" | grep -q "Adding " || \
       echo "$out" | grep -q "successfully added" || \
       echo "$out" | grep -q "uploaded successfully"; then
      log "Upload succeeded"
      success=true
      break
    fi
    
    log "Upload attempt $attempt failed: $out"
    attempt=$((attempt + 1))
    [[ $attempt -le $max_attempts ]] && sleep 2
  done
  
  if [[ $success == false ]]; then
    log "Failed to upload after $max_attempts attempts"
    exit 1
  fi
  
  # Verify the upload by checking sources
  log "Verifying upload..."
  if ! nlm sources "$id" | grep -q "$(basename "$pdf")"; then
    log "Warning: PDF not found in notebook sources after upload"
    # Don't exit here as the upload might still be processing
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