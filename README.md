# Domain Availability Checker

A fast, concurrent domain availability checking service and CLI written in Go. Uses modern RDAP protocol with intelligent fallback for maximum speed and reliability.

## Features

- **Web Dashboard**: Interactive web interface for domain checking (NEW in v2.1)
- **Fast RDAP Protocol**: 3-5x faster than traditional WHOIS (100-500ms vs 500-2000ms)
- **DNS Pre-filtering**: Quick availability checks via DNS nameserver queries (10-120ms)
- **Intelligent Fallback**: Automatic WHOIS fallback for domains without RDAP support
- **Bulk Processing**: Check up to 100 domains per request with concurrent processing
- **Security Hardened**: CSRF protection, XSS prevention, DoS mitigation, input validation
- **JSON API**: RESTful API for easy integration
- **Flexible CLI**: Multiple input methods (args, file, stdin) with filtering options
- **100% Backward Compatible**: Drop-in replacement for v1.x and v2.0

## Quick Start

```bash
# Build
go build -o domaincheck-server ./cmd/server
go build -o domaincheck ./cmd/cli

# Start server
./domaincheck-server

# Check domains
./domaincheck trucore priment axient
```

## Architecture

```
domaincheck/
├── cmd/
│   ├── cli/          # CLI client
│   └── server/       # HTTP server
├── internal/
│   ├── domain/       # Shared types and domain normalization
│   ├── checker/      # Domain availability checking logic
│   │   ├── dns.go    # DNS pre-filter (fastest, 10-120ms)
│   │   ├── rdap.go   # RDAP client (primary, 100-500ms)
│   │   └── whois.go  # WHOIS fallback (legacy, 200-2000ms)
│   └── server/       # HTTP handlers
```

### How It Works

1. **DNS Pre-filter** (10-120ms): Quick check for nameservers - if none exist, domain is likely available
2. **RDAP Query** (100-500ms): Modern protocol with structured JSON responses - 3-5x faster than WHOIS
3. **WHOIS Fallback** (200-2000ms): Legacy protocol for TLDs without RDAP support

## Requirements

- Go 1.21+
- `whois` command (optional, for fallback only)
  - macOS: `brew install whois`
  - Ubuntu: `apt install whois`

## Installation

### Build from Source

```bash
# Build both server and CLI
go build -o domaincheck-server ./cmd/server
go build -o domaincheck ./cmd/cli

# Or use make (if available)
make build
```

### Run Tests

```bash
# All tests
go test ./...

# With coverage
go test -cover ./...

# With race detection
go test -race ./...
```

## Usage

### Server

```bash
# Default port 8765
./domaincheck-server

# Custom port
PORT=9000 ./domaincheck-server

# Interactive mode (type domains directly)
./domaincheck-server
> trucore
✓ trucore.com - AVAILABLE (via rdap)
> quit
```

**Server Endpoints:**
- `GET /` - Web dashboard (interactive form)
- `POST /check` - Check multiple domains (JSON body)
- `GET /check/{domain}` - Check single domain
- `GET /health` - Health check

#### Web Dashboard

Open `http://localhost:8765/` in your browser to access the interactive dashboard.

**Features:**
- Real-time domain availability checking
- Bulk domain input (up to 100 domains)
- Visual results with status indicators
- Copy results to clipboard
- CSRF-protected form submissions
- Mobile-responsive design

**Security:**
- CSRF token protection on all form submissions
- XSS prevention via content sanitization
- Content Security Policy headers
- Input validation (max 100 domains)

### CLI

**Basic Usage:**

```bash
# Single domain
./domaincheck trucore.com

# Multiple domains (auto-adds .com if no TLD)
./domaincheck trucore priment axient vericor

# From file (one domain per line)
./domaincheck -f domains.txt

# From stdin
echo -e "trucore\npriment\naxient" | ./domaincheck -

# Show only available domains
./domaincheck -a trucore priment axient valcor

# JSON output
./domaincheck -j trucore priment

# Quiet mode (exit code only: 0=available, 1=taken/error)
./domaincheck -q trucore && echo "Available!" || echo "Taken"

# Custom server
./domaincheck -s http://api.example.com:9000 trucore
```

**CLI Options:**

| Option | Description |
|--------|-------------|
| `-s <url>` | Server URL (default: http://localhost:8765) |
| `-f <file>` | Read domains from file (max 10MB) |
| `-` | Read domains from stdin (max 10MB) |
| `-j` | Output raw JSON |
| `-a` | Show only available domains |
| `-q` | Quiet mode (exit code: 0=available, 1=taken) |
| `-h` | Show help |

### API Examples

**Check Single Domain (GET):**

```bash
curl http://localhost:8765/check/trucore.com
```

Response:
```json
{
  "results": [
    {"domain": "trucore.com", "available": true}
  ],
  "checked": 1,
  "available": 1,
  "taken": 0,
  "errors": 0
}
```

**Check Multiple Domains (POST):**

```bash
curl -X POST http://localhost:8765/check \
  -H "Content-Type: application/json" \
  -d '{"domains": ["trucore", "priment", "axient", "vericor"]}'
```

Response:
```json
{
  "results": [
    {"domain": "trucore.com", "available": true},
    {"domain": "priment.com", "available": false},
    {"domain": "axient.com", "available": false},
    {"domain": "vericor.com", "available": true}
  ],
  "checked": 4,
  "available": 2,
  "taken": 2,
  "errors": 0
}
```

**Error Handling:**

```json
{
  "results": [
    {"domain": "invalid..domain", "available": false, "error": "invalid domain format"}
  ],
  "checked": 1,
  "available": 0,
  "taken": 0,
  "errors": 1
}
```

## Performance

| Method | Latency | Use Case |
|--------|---------|----------|
| DNS Pre-filter | 10-120ms | Quick "likely available" check |
| RDAP | 100-500ms | **Primary method** - fast, structured data |
| WHOIS | 200-2000ms | Fallback for unsupported TLDs |

**Concurrency:**
- Server: 10 concurrent domain checks
- Request timeout: 60 seconds
- Per-domain timeout: 10 seconds
- Bulk requests: Up to 100 domains

## Security Features

- **CSRF Protection**: Synchronizer token pattern with 1-hour expiration (v2.1)
- **XSS Prevention**: Content sanitization and CSP headers (v2.1)
- **Command Injection Protection**: Strict domain validation with regex
- **DoS Prevention**: Request body limits (1MB), timeouts (60s), file size limits (10MB), CSRF token limits (10,000)
- **Input Validation**: All user input sanitized and validated
- **Resource Limits**: Controlled concurrency, bounded memory usage
- **Error Sanitization**: No internal details exposed to clients
- **Security Headers**: X-Frame-Options, X-Content-Type-Options, CSP (v2.1)

## Domain Normalization

The service automatically normalizes domain inputs:

| Input | Normalized | Notes |
|-------|------------|-------|
| `trucore` | `trucore.com` | Auto-adds .com if no TLD |
| `TRUCORE.COM` | `trucore.com` | Lowercase conversion |
| `  trucore  ` | `trucore.com` | Whitespace trimming |
| `example.org` | `example.org` | Preserves existing TLD |
| `-badactor.com` | ❌ Rejected | Security: prevents flag injection |
| `invalid..domain` | ❌ Rejected | Invalid format |

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8765` | Server port |

### Timeouts

| Type | Value | Location |
|------|-------|----------|
| Request timeout | 60s | Server enforced |
| Per-domain timeout | 10s | Checker |
| CLI timeout | 12s per domain (min 30s, max 300s) | CLI client |

### Limits

| Limit | Value | Purpose |
|-------|-------|---------|
| Request body | 1MB | DoS prevention |
| Concurrent checks | 10 | Rate limiting |
| Max domains per request | 100 | Practical limit |
| Input file size | 10MB | CLI memory protection |

## Testing

The project includes comprehensive test coverage:

```bash
# Run all tests
go test ./...

# View coverage
go test -cover ./...

# Race detection
go test -race ./...
```

**Test Coverage:**
- `internal/domain`: 100% (normalization + security tests)
- `internal/checker`: 100% (mocked unit tests)
- `internal/server`: 95%+ (HTTP handlers, CSRF, XSS, dashboard tests)

**Total:** 212 tests (23 new dashboard tests in v2.1), 100% pass rate

## Troubleshooting

**Server won't start:**
```bash
# Check if port is in use
lsof -i :8765

# Use different port
PORT=9000 ./domaincheck-server
```

**CLI can't connect:**
```bash
# Verify server is running
curl http://localhost:8765/health

# Check server URL
./domaincheck -s http://localhost:8765 trucore
```

**Timeouts on bulk checks:**
```bash
# Reduce batch size (100 domains * 10s = 1000s potential)
# Break into smaller batches for better results
head -50 domains.txt | ./domaincheck -
```

## Migration from v1.x / v2.0

v2.1 is 100% backward compatible with v1.x and v2.0:

- ✅ All API endpoints unchanged
- ✅ All CLI flags work identically
- ✅ JSON response format unchanged
- ✅ Domain normalization behavior preserved (bug fixed: `example.org` no longer becomes `example.org.com`)

**What's New in v2.1:**
- Web dashboard at `http://localhost:8765/`
- CSRF protection for web form submissions
- XSS prevention and security headers
- Enhanced DoS protection (CSRF token limits)

**What's New in v2.0:**
- RDAP protocol for 3-5x speed improvement
- DNS pre-filter for quick checks
- Enhanced security hardening
- Improved error handling
- Better test coverage

**No breaking changes** - drop-in replacement.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Ensure all tests pass: `go test ./...`
5. Submit a pull request

## License

MIT License - see LICENSE file for details

## Project Status

**Version:** 2.1.0
**Status:** Production Ready
**Last Updated:** 2026-01-30

- ✅ Web dashboard implemented (v2.1)
- ✅ CSRF & XSS protection (v2.1)
- ✅ Core refactoring complete
- ✅ RDAP protocol implemented
- ✅ Security hardened (0 CRITICAL, 0 HIGH vulnerabilities)
- ✅ All tests passing (212 tests)
- ✅ 100% backward compatible with v1.x and v2.0

See [PROJECT-STATUS.md](PROJECT-STATUS.md) for detailed development progress.
