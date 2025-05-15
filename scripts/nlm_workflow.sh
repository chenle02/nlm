#!/usr/bin/env bash
# Determine the directory of this script
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Automate NotebookLM workflow with nlm CLI
# Usage: ./nlm_workflow.sh <document.pdf>
# Requirements: nlm CLI configured & authenticated

set -euo pipefail
IFS=$'\n\t'

# CSV file path
CSV_FILE="${script_dir}/notebooks.csv"

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

# Initialize CSV file if it doesn't exist
init_csv() {
  if [[ ! -f "$CSV_FILE" ]]; then
    log "Creating new CSV file at $CSV_FILE"
    echo "id,title,file_count" > "$CSV_FILE"
  fi
}

# Update CSV with current notebook list
update_csv() {
  log "Updating notebook CSV..."
  # Create temporary file
  local temp_file
  temp_file=$(mktemp)
  
  # Write header
  echo "id,title,file_count" > "$temp_file"
  
  # Get notebook list and process each line
  nlm list 2>/dev/null | tail -n +3 | while read -r line; do
    # Extract ID and title
    local id title
    id=$(echo "$line" | grep -Eo '[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}' | head -1)
    title=$(echo "$line" | sed -E 's/^[[:space:]]*//' | sed -E 's/[[:space:]]*$//' | sed -E "s/$id//" | sed -E 's/^[[:space:]]*//')
    
    if [[ -n "$id" && -n "$title" ]]; then
      # Get file count for this notebook
      local file_count
      file_count=$(nlm sources "$id" 2>/dev/null | wc -l)
      file_count=$((file_count - 1)) # Subtract header line
      [[ $file_count -lt 0 ]] && file_count=0
      
      echo "$id,$title,$file_count" >> "$temp_file"
    fi
  done
  
  # Replace old CSV with new one
  mv "$temp_file" "$CSV_FILE"
  log "CSV file updated successfully"
}

# Get notebook ID from CSV by title
get_notebook_id() {
  local title="$1"
  local id
  id=$(awk -F, -v title="$title" '$2 == title {print $1}' "$CSV_FILE" | head -1)
  echo "$id"
}

# Check if notebook exists in CSV
check_notebook_exists() {
  local title="$1"
  log "Checking if notebook '$title' exists in CSV..."
  
  # First update the CSV to ensure we have latest data
  update_csv
  
  # Debug output
  log "Available notebooks in CSV:"
  tail -n +2 "$CSV_FILE" | while IFS=, read -r id name count; do
    log "  '$name' (ID: $id, Files: $count)"
  done
  
  # Check for exact title match
  if awk -F, -v title="$title" '$2 == title {exit 1}' "$CSV_FILE"; then
    log "Found notebook '$title' in CSV"
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
  
  # Update CSV with new notebook
  update_csv
  
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
  init_csv

  local base
  base=$(basename "${pdf%.*}")

  if check_notebook_exists "$base"; then
    log "Notebook '$base' already exists. Exiting."
    exit 0
  fi

  # Create notebook and get its ID
  nb_id=$(create_notebook "$base")
  upload_pdf "$nb_id" "$pdf"
  
  # Update CSV after upload
  update_csv

  # Generate and download audio overview (.wav)
  local outfile="${script_dir}/${base}.wav"
  generate_and_download_audio "$nb_id" "$outfile"

  log "Workflow complete. Audio saved to '$outfile'"
  log "Notebook tracking CSV updated at $CSV_FILE"
}

main "$@"