# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

AI-based bot for Discord, written in Go.

## Documentation

All specifications, architecture decisions, and development history are tracked in Notion:
- Main page: https://www.notion.so/2c9343c710ae81f09a4dca0c3c40e2a6
- History database tracks: Development, Conversations, Decisions, Bugs, Features, Refactors

When making significant changes, update the Notion History database with:
- Summary (brief, under 100 words)
- Context (background, no code)
- Outcome (result)
- Appropriate tags

## Build Commands

```bash
go build ./...           # Build all packages
go test ./...            # Run all tests
go test -v ./... -run TestName  # Run specific test
go mod tidy              # Clean up dependencies
```

## Environment

Uses `.env` file for configuration (gitignored).
