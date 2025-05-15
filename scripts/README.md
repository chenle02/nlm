 # Scripts Directory

 This directory contains example PDF files and automation scripts for the NotebookLM CLI (`nlm`).

 ## Files
 - `nlm_workflow.sh`: Automates the following workflow:
   1. Extracts the notebook title from a PDF filename.
   2. Checks if a notebook with that title exists (`nlm list`).
   3. If not, creates the notebook and uploads the PDF (`nlm add`).
   4. Generates an audio overview (`nlm audio-create`) and polls until it is ready.
   5. Downloads the resulting audio file (`nlm audio-get`).

 - Sample PDFs (`chen_*.pdf`, `dummy*.pdf`): Test source files for the workflow.
 - `notebooks.csv`: Optional CSV mapping PDF filenames to existing notebook IDs.

 ## Dependencies
 - `nlm` CLI (built from this repo): see root `README.md`.
 - `pdftotext`: used as a fallback for PDF ingestion when binary uploads fail.
 - `bash`, `grep`, `jq`, `sleep`, and other standard Unix utilities.

 ## Usage

 1. Authenticate with NotebookLM:
    ```bash
    nlm auth
    ```
 2. Run the workflow script against a PDF:
    ```bash
    ./nlm_workflow.sh path/to/document.pdf
    ```

 The script logs progress, surfaces errors (e.g., daily audio limits), and saves any generated audio files to the current directory.

 ## Changelog (v0.1)
 - Fallback polling for PDF upload responses that initially return `null`.
 - Automatic text-extraction fallback via `pdftotext` when binary uploads repeatedly fail.
 - Enhanced `audio-create` error handling to report daily generation limits.
 - Added `-debug` flag to the CLI for inspecting batchexecute RPC calls.