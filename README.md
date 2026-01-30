# pocketbase-importer

Small, fast, external-dependency-free CLI tool to bulk-import CSV files into **PocketBase** collections.

Handles large files, messy CSVs, missing values.

## Features

- Efficient parallel processing (configurable number of goroutines)
- Good handling of problematic CSVs:
  - multiline quoted values
  - custom delimiters
  - irregular number of columns
- Creates missing IDs/field values automatically when PocketBase is configured to auto-generate them
- Skips "unique constraint" errors by default (useful when re-running imports)
- Real-time progress: shows processed rows + rows/second
- No external dependencies besides PocketBase itself
- Optional record validation before insert (`-validate=true/false`) (*Note*: will slow down import speed)

## Requirements

- Go 1.25.5+ (only if building yourself)
- PocketBase v0.36+ compatible data directory

## Installation

### Option 1: Use prebuilt binaries (recommended)

Download from [GitHub Releases](https://github.com/jim-ww/pocketbase-importer/releases)

Available for:
- Linux (amd64, arm64)
- macOS (amd64, arm64)
- Windows (amd64)

### Option 2: Build from source

```bash
go install github.com/jim-ww/pocketbase-importer@latest
```
