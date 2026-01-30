# Domain Availability Checker

A fast, concurrent domain availability checking service and CLI written in Go.

## Features

- Check single or bulk domains (up to 100 per request)
- Concurrent checking with rate limiting (10 parallel whois queries)
- JSON API for integration
- CLI tool with multiple input methods
- Filters for available-only results

## Requirements

- Go 1.21+
- `whois` command installed (`brew install whois` on macOS, `apt install whois` on Ubuntu)

## Build

```bash
# Build both server and CLI
make build

# Or build individually
go build -o domaincheck-server ./cmd/server
go build -o domaincheck ./cmd/cli
```

## Usage

### Start the Server

```bash
# Default port 8765
./domaincheck-server

# Custom port
PORT=9000 ./domaincheck-server
```

### CLI Examples

```bash
# Single domain
./domaincheck trucore.com

# Multiple domains (auto-adds .com)
./domaincheck trucore priment axient vericor

# From file
./domaincheck -f domains.txt

# From stdin
echo -e "trucore\npriment\naxient" | ./domaincheck -

# Show only available domains
./domaincheck -a trucore priment axient valcor

# JSON output
./domaincheck -j trucore priment

# Quiet mode (exit code only)
./domaincheck -q trucore && echo "Available!" || echo "Taken"
```

### API Examples

```bash
# Check single domain via GET
curl http://localhost:8765/check/trucore.com

# Check multiple domains via POST
curl -X POST http://localhost:8765/check \
  -H "Content-Type: application/json" \
  -d '{"domains": ["trucore", "priment", "axient", "vericor"]}'
```

### Response Format

```json
{
  "results": [
    {"domain": "trucore.com", "available": true},
    {"domain": "priment.com", "available": false},
    {"domain": "axient.com", "available": false, "error": "timeout"}
  ],
  "checked": 3,
  "available": 1,
  "taken": 1,
  "errors": 1
}
```

## CLI Options

| Option | Description |
|--------|-------------|
| `-s <url>` | Server URL (default: http://localhost:8765) |
| `-f <file>` | Read domains from file |
| `-` | Read domains from stdin |
| `-j` | Output raw JSON |
| `-a` | Show only available domains |
| `-q` | Quiet mode (exit code: 0=available, 1=taken) |
| `-h` | Show help |

## Notes

- Domains without a TLD automatically get `.com` appended
- The service uses the system's `whois` command
- Rate limited to 10 concurrent queries to avoid overwhelming whois servers
- Timeout is 10 seconds per domain
