# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

This is a command-line interface (CLI) tool for Google's NotebookLM. It's a fork of [tmc/nlm](https://github.com/tmc/nlm) with significant fixes and enhancements, including support for audio/video overviews, improved authentication, and better response parsing.

## Key Build and Development Commands

```bash
# Build the main binary
go build ./cmd/nlm

# Install to GOPATH/bin
go install ./cmd/nlm

# Run tests for specific packages
go test ./internal/batchexecute
go test ./internal/beprotojson

# Run all tests
go test ./...

# Run with debug output
./nlm -debug <command>

# Test authentication
./nlm -debug auth

# Test basic functionality
./nlm list
```

## Architecture Overview

### Core Components

**RPC Layer (`internal/rpc/`)**
- Contains hardcoded RPC endpoint IDs for NotebookLM's internal APIs
- Each operation (create, list, delete, etc.) maps to a specific endpoint ID (e.g., "wXbhsf" for ListRecentlyViewedProjects)
- Critical: Video overview endpoints use placeholder values ("UNKNOWN1", etc.) that need discovery via network inspection

**BatchExecute Client (`internal/batchexecute/`)**
- Handles Google's batchexecute protocol used by NotebookLM web interface  
- Key issue fixed: infinite recursion in response parsing that caused empty results
- Supports both regular and chunked response formats
- Contains response decoding logic that strips JSON prefixes like ")]}"

**API Client (`internal/api/`)**
- High-level wrapper around RPC calls
- Notable: `ListRecentlyViewedProjects()` uses custom parsing instead of protobuf due to API format changes
- Audio/Video overview support with polling for completion status

**Authentication (`internal/auth/`)**
- Browser-based OAuth flow using ChromeDP
- Cross-platform Chrome profile detection (Windows, macOS, Linux)
- Stores credentials in `~/.nlm/env` file

**Protocol Buffers (`gen/notebooklm/v1alpha1/`)**
- Generated protobuf definitions (may not match current API exactly)
- Custom JSON unmarshaling in `internal/beprotojson/` for Google's wire format

### Critical Implementation Details

**Response Parsing Pattern**
NotebookLM API responses follow this pattern:
1. Strip prefix `")]}'` or `)]}`
2. Handle chunked encoding with length prefixes  
3. Parse JSON array format: `[["wrb.fr", "endpoint_id", "data", ...]]`

**RPC Endpoint Discovery**
When adding new functionality:
1. Use NotebookLM web interface with browser dev tools
2. Find batchexecute network requests
3. Extract 6-character endpoint IDs from request body
4. Update constants in `internal/rpc/rpc.go`

**Authentication Flow**
- Uses Chrome with temporary profile copying
- Extracts cookies and auth tokens via DOM inspection
- Supports custom browser profiles via `NLM_BROWSER_PROFILE` env var

## Automated Workflow Scripts

**`scripts/nlm_workflow.sh`**
- End-to-end automation: PDF → notebook → audio overview
- Features smart duplicate detection and retry logic
- Usage: `./scripts/nlm_workflow.sh document.pdf`

## Development Notes

**Known Issues**
- Video overview functionality implemented but needs RPC endpoint discovery
- Some protobuf definitions may be outdated due to API evolution
- Authentication requires GUI environment for Chrome automation

**Testing Authentication**
- Remove `~/.nlm/env` to test fresh auth flow
- Use `-debug` flag to see detailed RPC traffic
- Confirmed working on Ubuntu Linux

**Fork-Specific Enhancements**
- Fixed infinite recursion in batchexecute response parsing
- Custom ListRecentlyViewedProjects parser for API format changes
- Improved timeout handling in authentication
- Added video overview framework (pending endpoint discovery)