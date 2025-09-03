#!/usr/bin/env bash
set -euo pipefail

# Helper to authenticate nlm using your running Google Chrome session.
# Flow:
# 1) Opens NotebookLM in Chrome with your chosen profile.
# 2) You log in (if needed), open DevTools → Network, copy a batchexecute request as cURL.
# 3) Script reads that cURL from the clipboard (or stdin) and feeds it to `nlm auth`.

echo "[nlm-auth] Starting guided authentication for NotebookLM..."

# Resolve nlm binary (prefer local, then PATH)
NLM_BIN=""
if [[ -x "$(pwd)/nlm" ]]; then
  NLM_BIN="$(pwd)/nlm"
elif command -v nlm >/dev/null 2>&1; then
  NLM_BIN="$(command -v nlm)"
fi
if [[ -z "${NLM_BIN}" ]]; then
  echo "[nlm-auth] Error: 'nlm' binary not found in current dir or PATH." >&2
  exit 1
fi

# Pick Chrome profile
PROFILE="${NLM_BROWSER_PROFILE:-Default}"
echo "[nlm-auth] Using Chrome profile: ${PROFILE}  (override with NLM_BROWSER_PROFILE)"

# Find Chrome
CHROME_BIN=""
for name in google-chrome chrome chromium; do
  if command -v "$name" >/dev/null 2>&1; then
    CHROME_BIN="$(command -v "$name")"
    break
  fi
done
if [[ -z "${CHROME_BIN}" ]]; then
  echo "[nlm-auth] Error: Google Chrome/Chromium not found on PATH." >&2
  exit 1
fi

# Open NotebookLM in your normal Chrome window
echo "[nlm-auth] Launching Chrome → https://notebooklm.google.com"
"${CHROME_BIN}" --profile-directory="${PROFILE}" "https://notebooklm.google.com" >/dev/null 2>&1 &
sleep 2

cat << 'INSTR'

Instructions:
  1) Complete Google sign-in in the opened Chrome window (if prompted).
  2) Open DevTools (Ctrl+Shift+I) → Network tab.
  3) Refresh the page and filter for: batchexecute
  4) Right-click a batchexecute request → Copy → Copy as cURL (bash).
  5) Return here and press Enter. The script will read your clipboard and finish auth.

If clipboard tools are unavailable, paste the copied cURL here, then press Ctrl+D.
INSTR

read -r -p "Press Enter once you've copied the cURL (or paste it now)..." || true

# Read cURL from clipboard if possible; else from stdin.
read_curl() {
  if command -v xclip >/dev/null 2>&1; then
    xclip -o -selection clipboard
    return 0
  elif command -v xsel >/dev/null 2>&1; then
    xsel --clipboard --output
    return 0
  elif command -v wl-paste >/dev/null 2>&1; then
    wl-paste -n
    return 0
  else
    # Fallback: read from stdin until EOF
    cat
    return 0
  fi
}

CURL_CMD="$(read_curl || true)"

if [[ -z "${CURL_CMD//[$'\t\r\n ']}" ]]; then
  echo "[nlm-auth] Error: No cURL command captured from clipboard/stdin." >&2
  exit 1
fi

# Normalize Cookie header case so parser is robust (handles 'Cookie:' and 'cookie:').
NORMALIZED_CMD=$(printf '%s' "${CURL_CMD}" \
  | sed -E "s/((^|[[:space:]])(-H|--header)[[:space:]]+)(['\"])Cookie:/\1\4cookie:/Ig")

echo "[nlm-auth] Feeding captured cURL to nlm..."
set +e
AUTH_OUT=$(printf '%s' "${NORMALIZED_CMD}" | "${NLM_BIN}" auth 2>&1)
STATUS=$?
set -e

echo "${AUTH_OUT}" >&2
if [[ ${STATUS} -ne 0 ]]; then
  echo "[nlm-auth] Authentication failed. Ensure the cURL includes cookie header and 'at=' token." >&2
  exit ${STATUS}
fi

echo "[nlm-auth] Done. Saved credentials to ~/.nlm/env"
echo "[nlm-auth] Try: nlm list"

