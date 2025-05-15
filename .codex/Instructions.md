# 📘 Codex Project Instructions: NLM

## 🎯 Objective

To design, develop, and maintain a robust, user-friendly, and extensible command-line interface (CLI) for interacting with **Google NotebookLM**. The goal is to support seamless workflows from the terminal for:

- Document ingestion  
- Intelligent note generation  
- Audio (podcast-style) synthesis  

This CLI aims to empower researchers, content creators, and developers to automate their use of NotebookLM efficiently and reproducibly.

---

## ✅ Project Goals

1. [x] **Fix core compatibility issues** with the upstream `nlm` tool, including:
   - JSON decoding for large/multi-chunk responses
   - Browser-based authentication flow

2. [ ] **Implement a clean and ergonomic CLI** for:
   - Uploading and managing documents
   - Generating summaries and notes
   - Producing and downloading audio podcasts

---

## 📂 Requirements

- All automation scripts should be placed under the `scripts/` directory.
- A `README.md` must be maintained inside the `scripts/` folder. It should clearly document:
  - Script purpose and usage
  - Required dependencies
  - Environment variables (if any)
  - Known limitations
  - Revision history or changelog (brief)
