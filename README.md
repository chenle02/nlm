> **🚨 Fork Notice**
> 
> This repository is a personal fork of [tmc/nlm](https://github.com/tmc/nlm), an experimental command-line interface for Google's [NotebookLM](https://notebooklm.google.com/).
> 
> The purpose of this fork is to:
> - Fix JSON unmarshaling issues affecting `nlm list`
> - Improve cross-platform compatibility and usability
> - Extend functionality for podcast automation and terminal workflows
> 
> Original author: [@tmc](https://github.com/tmc)  
> Fork maintained by: [@chenle02](https://github.com/chenle02)

---
# nlm - NotebookLM CLI Tool 📚

`nlm` is a command-line interface for Google's NotebookLM, allowing you to manage notebooks, sources, and audio overviews from your terminal.

🔊 Listen to an Audio Overview of this tool here: [https://notebooklm.google.com/notebook/437c839c-5a24-455b-b8da-d35ba8931811/audio](https://notebooklm.google.com/notebook/437c839c-5a24-455b-b8da-d35ba8931811/audio).

## Installation 🚀

```bash
git clone https://github.com/chenle02/nlm
cd nlm
go install ./cmd/nlm
```

The `nlm` binary will be installed to `$(go env GOPATH)/bin`. Ensure this directory is included in your `PATH`:

```bash
export PATH="$(go env GOPATH)/bin:$PATH"
```

### Usage 

```shell
Usage: nlm <command> [arguments]

Notebook Commands:
  list, ls          List all notebooks
  create <title>    Create a new notebook
  rm <id>           Delete a notebook
  analytics <id>    Show notebook analytics

Source Commands:
  sources <id>      List sources in notebook
  add <id> <input>  Add source to notebook
  rm-source <id> <source-id>  Remove source
  rename-source <source-id> <new-name>  Rename source
  refresh-source <source-id>  Refresh source content
  check-source <source-id>  Check source freshness

Note Commands:
  notes <id>        List notes in notebook
  new-note <id> <title>  Create new note
  edit-note <id> <note-id> <content>  Edit note
  rm-note <note-id>  Remove note

Audio Commands:
  audio-create <id> <instructions>  Create audio overview
  audio-get <id>    Get audio overview
  audio-rm <id>     Delete audio overview
  audio-share <id>  Share audio overview

Video Commands:
  video-create <id> <instructions>  Create video overview
  video-get <id>    Get video overview
  video-rm <id>     Delete video overview

Generation Commands:
  generate-guide <id>  Generate notebook guide
  generate-outline <id>  Generate content outline
  generate-section <id>  Generate new section

Other Commands:
  auth              Setup authentication
```

<details>
<summary>📦 Installing Go (if needed)</summary>

### Option 1: Using Package Managers

**macOS (using Homebrew):**
```bash
brew install go
```

**Linux (Ubuntu/Debian):**
```bash
sudo apt update
sudo apt install golang
```

**Linux (Fedora):**
```bash
sudo dnf install golang
```

### Option 2: Direct Download

1. Visit the [Go Downloads page](https://go.dev/dl/)
2. Download the appropriate version for your OS
3. Follow the installation instructions:

**macOS:**
- Download the .pkg file
- Double-click to install
- Follow the installer prompts

**Linux:**
```bash
# Example for Linux AMD64 (adjust version as needed)
wget https://go.dev/dl/go1.21.6.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.21.6.linux-amd64.tar.gz
```

### Post-Installation Setup

Add Go to your PATH by adding these lines to your `~/.bashrc`, `~/.zshrc`, or equivalent:
```bash
export PATH=$PATH:/usr/local/go/bin
export PATH=$PATH:$(go env GOPATH)/bin
```

Verify installation:
```bash
go version
```
</details>

## Authentication 🔑

First, authenticate with your Google account:

```bash
nlm auth
```

This will launch Chrome to authenticate with your Google account. The authentication tokens will be saved in `~/.nlm/env` file.

If you encounter issues during authentication, try using debug mode for additional troubleshooting information:

```bash
nlm -debug auth
```

### Manual Authentication (Recommended)

If automatic browser authentication fails or captures incomplete cookies, use the manual cURL method:

1. Open https://notebooklm.google.com in Chrome (make sure you're logged in)
2. Open DevTools (F12) → Network tab
3. Refresh the page
4. Find any `batchexecute` request
5. Right-click → **Copy as cURL**
6. Run:
   ```bash
   echo '<paste curl command here>' | ./nlm auth
   ```

This extracts the proper authenticated cookies and auth token from your browser session.

**Note**: Authentication has been tested and confirmed working on Ubuntu Linux. The tool will automatically launch Chrome in the background to handle the Google OAuth flow.

## Usage 💻

### Notebook Operations

```bash
# List all notebooks
nlm list

# Create a new notebook
nlm create "My Research Notes"

# Delete a notebook
nlm rm <notebook-id>

# Get notebook analytics
nlm analytics <notebook-id>
```

### Source Management

```bash
# List sources in a notebook
nlm sources <notebook-id>

# Add a source from URL
nlm add <notebook-id> https://example.com/article

# Add a source from file
nlm add <notebook-id> document.pdf

# Add source from stdin
echo "Some text" | nlm add <notebook-id> -

# Rename a source
nlm rename-source <source-id> "New Title"

# Remove a source
nlm rm-source <notebook-id> <source-id>
```

### Note Operations

```bash
# List notes in a notebook
nlm notes <notebook-id>

# Create a new note
nlm new-note <notebook-id> "Note Title"

# Edit a note
nlm edit-note <notebook-id> <note-id> "New content"

# Remove a note
nlm rm-note <note-id>
```

### Audio Overview

```bash
# Create an audio overview
nlm audio-create <notebook-id> "speak in a professional tone"

# Get audio overview status/content
nlm audio-get <notebook-id>

# Share audio overview (private)
nlm audio-share <notebook-id>

# Share audio overview (public)
nlm audio-share <notebook-id> --public
```

### Video Overview

```bash
# Create a video overview
nlm video-create <notebook-id> "create engaging visual explanation"

# Get video overview status/content
nlm video-get <notebook-id>

# Delete video overview
nlm video-rm <notebook-id>
```

**Note**: Video overview functionality requires discovery of the actual NotebookLM RPC endpoints. Currently, the video commands are implemented but will show an informative error message until the correct RPC endpoint IDs are discovered through network inspection of the NotebookLM web interface.

## Examples 📋

Create a notebook and add some content:
```bash
# Create a new notebook
notebook_id=$(nlm create "Research Notes" | grep -o 'notebook [^ ]*' | cut -d' ' -f2)

# Add some sources
nlm add $notebook_id https://example.com/research-paper
nlm add $notebook_id research-data.pdf

# Create an audio overview
nlm audio-create $notebook_id "summarize in a professional tone"

# Create a video overview
nlm video-create $notebook_id "create visual presentation with key insights"

# Check the audio overview
nlm audio-get $notebook_id

# Check the video overview
nlm video-get $notebook_id
```

### Automated Workflow Script 🤖

For automated PDF processing, use the included workflow script:

```bash
# Run the automated workflow
./scripts/nlm_workflow.sh document.pdf
```

**What it does:**
- Creates a notebook named after the PDF file (if it doesn't exist)
- Uploads the PDF as a source to the notebook
- Generates an audio overview automatically
- Downloads the audio file to the current directory as `document.wav`

**Features:**
- ✅ Smart duplicate detection - skips processing if notebook already exists
- ✅ Robust upload with retry logic and verification
- ✅ Automatic polling for audio readiness
- ✅ Clean output with timestamped logging

**Example usage:**
```bash
# Process a research paper
./scripts/nlm_workflow.sh research-paper-2024.pdf

# The script will:
# 1. Create notebook "research-paper-2024"
# 2. Upload the PDF
# 3. Generate audio overview
# 4. Save as "research-paper-2024.wav"
```

**Requirements:**
- nlm CLI must be installed and authenticated (`nlm auth`)
- Standard Unix tools: `grep`, `sed`, `head`, `mktemp`

## Advanced Usage 🔧

### Debug Mode

Add `-debug` flag to see detailed API interactions:

```bash
nlm -debug list
```

### Environment Variables

- `NLM_AUTH_TOKEN`: Authentication token (stored in ~/.nlm/env)
- `NLM_COOKIES`: Authentication cookies (stored in ~/.nlm/env)
- `NLM_BROWSER_PROFILE`: Chrome profile to use for authentication (default: "Default")

These are typically managed by the `auth` command, but can be manually configured if needed.

## Contributing 🤝

Contributions are welcome! Please feel free to submit a Pull Request.

## 🚀 Enhancements in This Fork

### v0.2.0 (January 2026)
- ✅ **Fixed authentication error detection** - Now properly detects Google API error code 16 (UNAUTHENTICATED) and triggers re-auth
- ✅ **Updated API parameters** - Synced `bl` (build label) and `f.sid` parameters with current NotebookLM API
- ✅ **Improved auth token parsing** - Fixed URL-encoded token handling from cURL commands
- ✅ **Added cookie validation** - Warns when captured cookies are incomplete (missing SID, SSID, etc.)
- ✅ **Manual cURL auth method** - Added reliable fallback authentication via browser DevTools

### v0.1.0 (Previous)
- ✅ Fixed multi-chunk response decoding in `nlm list`
- ✅ Fixed infinite recursion bug in batchexecute response parsing
- ✅ Improved authentication timeout handling and reliability
- ✅ Added custom parser for NotebookLM API response format changes
- ✅ Confirmed working authentication on Ubuntu Linux with debug mode support
- ✅ Added Video Overview functionality framework (`video-create`, `video-get`, `video-rm`)
  - Framework ready for easy endpoint discovery and integration
  - Informative error messages explaining current limitations

## License 📄

MIT License - see [LICENSE](LICENSE) for details.
