# Scriptling CLI

A command-line interface for the Scriptling programming language.

## Installation

Download the appropriate binary for your platform from the releases or build from source.

## Usage

### Run a script file
```bash
scriptling script.py
```

### Run from stdin
```bash
echo 'print("Hello World")' | scriptling
```

### Interactive mode
```bash
scriptling --interactive
# or
scriptling -i
```

### Help
```bash
scriptling --help
```

## Building

The CLI tool uses [Task](https://taskfile.dev/) for building. Install Task first:

```bash
# macOS
brew install go-task/tap/go-task

# Or download from https://taskfile.dev/
```

### Build for current platform
```bash
task build
```

### Build for all platforms
```bash
task build-all
```

### Install locally
```bash
task install
```

## Features

- **File execution**: Run Scriptling scripts from files
- **Stdin execution**: Pipe scripts to stdin
- **Interactive mode**: REPL-like interactive execution
- **Cross-platform**: Built for Linux, macOS, and Windows on AMD64 and ARM64
- **Minimal size**: Optimized with stripped binaries (~7MB)

## Libraries

The CLI includes all standard libraries plus external libraries:
- `datetime`, `json`, `math`, `random`, `re`, `time`, `base64`, `hashlib`, `urllib`
- `requests` - HTTP client library