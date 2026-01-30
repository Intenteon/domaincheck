# Domain Checker Refactoring Implementation Plan

**Project:** Domain Availability Checker
**Goal:** Eliminate code duplication between server and CLI modes
**Status:** Planning Phase - Awaiting Approval
**Date:** 2026-01-30

---

## Executive Summary

This refactoring will eliminate ~25% code duplication between server and CLI modes by extracting shared logic into internal packages, while simultaneously upgrading from deprecated WHOIS to modern RDAP protocol. All changes maintain 100% backward compatibility with existing API and CLI interfaces.

### Key Changes Approved
1. ✅ Use CLI normalization logic (add .com only if no dot exists)
2. ✅ Upgrade to RDAP as primary protocol with DNS pre-filter and WHOIS fallback
3. ✅ Use `internal/` packages structure (Go best practice)
4. ✅ Remove interactive server stdin mode (reduce duplication)

### Expected Benefits
- **3-5x faster** domain checks (RDAP vs WHOIS)
- **Zero code duplication** in type definitions
- **Unified normalization** logic (fixes existing bug)
- **Better testability** through interface-based design
- **Future-proof** using modern RDAP standard (WHOIS deprecated Jan 2025)

---

## Research Findings Summary

### Product Requirements Guardian
- Identified 3 critical user-facing contracts that MUST NOT change:
  - API endpoints: `POST /check`, `GET /check/{domain}`, `GET /health`
  - CLI flags: `-s`, `-f`, `-j`, `-a`, `-q`, `-h`
  - JSON response structure (field names, types, nesting)
- Found domain normalization bug: Server corrupts non-.com domains (e.g., `example.org` → `example.org.com`)
- Documented 100% backward compatibility requirements

### Project Research Analyst
- **Critical finding:** WHOIS protocol officially deprecated January 28, 2025
- Confirmed RDAP is the modern replacement with:
  - Verisign RDAP endpoint: `https://rdap.verisign.com/com/v1/`
  - Simple HTTP/JSON interface
  - HTTP 404 = domain available, HTTP 200 = domain taken
  - No authentication required for .com lookups
- DNS NS lookup provides 10-120ms pre-filter vs 200-2000ms WHOIS
- Recommended Go libraries: Raw HTTP for RDAP, stdlib for DNS, `github.com/likexian/whois` for fallback

### Go Systems Expert
- Analyzed current architecture: Two monolithic entry points with duplicated logic
- Identified specific duplications:
  - Type definitions (CheckRequest, DomainResult, CheckResponse) - 100% identical in both files
  - Domain normalization logic - 90% similar with critical bug
  - WHOIS checking logic - Server-only, should be extracted
- Designed clean `internal/` package structure:
  - `internal/domain/` - types and normalization
  - `internal/checker/` - checking logic with interfaces
  - `internal/server/` - HTTP handlers
  - `internal/config/` - shared constants
- Recommended interface-based design for testability with mocks

### UI/UX Design Specialist
- Recommended keeping CLI output format unchanged for backward compatibility
- Proposed optional `-v` verbose flag for enhanced output with timing/method info
- Designed structured error code system (TIMEOUT, RATE_LIMITED, etc.)
- Suggested additive API versioning (add fields, don't break existing ones)
- Provided migration path for interactive server mode removal

### Agreement Points
All three agents agree:
1. Extract shared types to `internal/domain/types.go` (zero risk, high value)
2. Implement checker interfaces for testability
3. Upgrade to RDAP with DNS pre-filter and WHOIS fallback
4. Maintain 100% backward compatibility
5. Remove interactive server mode
6. Use CLI normalization logic as standard

---

## Current State Analysis

### Existing Codebase Structure
```
domaincheck/
├── cmd/
│   ├── cli/main.go          (236 lines) - CLI client
│   └── server/main.go       (237 lines) - HTTP server
├── go.mod                   - No external dependencies
├── Makefile                 - Build automation
└── README.md               - Documentation
```

### Code Duplication Points

| Component | Server Location | CLI Location | Lines | Priority |
|-----------|----------------|--------------|-------|----------|
| Type definitions | Lines 23-39 | Lines 16-32 | ~17 | HIGH |
| Domain normalization | Lines 43-46 | Lines 145-151 | ~7 | HIGH (has bug) |
| Domain checking logic | Lines 41-115 | N/A | ~75 | MEDIUM |
| Output formatting | Lines 223-229 | Lines 216-229 | ~14 | LOW |

**Total duplication:** ~100 lines (~21% of codebase)

### Critical Bug Identified

**Location:** `cmd/server/main.go:43-46` vs `cmd/cli/main.go:145-151`

**Issue:** Server uses `!strings.HasSuffix(domain, ".com")` which corrupts non-.com TLDs:
- Input: `example.org` → Output: `example.org.com` (WRONG)

**Fix:** Use CLI logic `!strings.Contains(d, ".")` which preserves valid TLDs:
- Input: `example.org` → Output: `example.org` (CORRECT)
- Input: `example` → Output: `example.com` (CORRECT)

---

## Target Architecture

### Package Structure
```
domaincheck/
├── cmd/
│   ├── cli/
│   │   └── main.go              # CLI entry point (~150 lines)
│   └── server/
│       └── main.go              # Server entry point (~80 lines)
├── internal/
│   ├── domain/
│   │   ├── types.go             # Shared types (Domain, Result, Status)
│   │   ├── normalize.go         # Domain normalization logic
│   │   └── normalize_test.go    # Normalization tests
│   ├── checker/
│   │   ├── checker.go           # Main Checker interface and implementation
│   │   ├── checker_test.go      # Checker tests with mocks
│   │   ├── rdap.go              # RDAP client implementation
│   │   ├── rdap_test.go         # RDAP client tests
│   │   ├── whois.go             # WHOIS fallback implementation
│   │   ├── whois_test.go        # WHOIS tests
│   │   ├── dns.go               # DNS NS pre-filter
│   │   └── dns_test.go          # DNS tests
│   ├── server/
│   │   ├── handlers.go          # HTTP handlers
│   │   └── handlers_test.go     # Handler tests
│   └── config/
│       └── config.go            # Shared configuration
├── go.mod
├── go.sum                       # Will appear after adding dependencies
├── Makefile
└── README.md
```

### Interface Design

```go
// Checker - Main domain availability checker
type Checker interface {
    Check(ctx context.Context, domainName string) domain.Result
    CheckBatch(ctx context.Context, domains []string) []domain.Result
}

// RDAPClient - RDAP protocol client
type RDAPClient interface {
    Lookup(ctx context.Context, d domain.Domain) (*RDAPResponse, error)
}

// WHOISClient - WHOIS fallback client
type WHOISClient interface {
    Query(ctx context.Context, d domain.Domain) (string, error)
}

// DNSFilter - Fast DNS pre-filter
type DNSFilter interface {
    HasNameservers(ctx context.Context, d domain.Domain) (bool, error)
}
```

### Domain Checking Flow
```
User Input → Normalize → DNS Pre-filter → RDAP Lookup → WHOIS Fallback → Result
                            ↓                   ↓              ↓
                       (10-120ms)         (100-500ms)    (200-2000ms)
```

---

## Implementation Task List

### Phase 1: Foundation & Types (Zero Risk)

#### TASK-001: Create internal/domain package structure
**Owner:** Ollama (initial) → go-systems-expert (review)
**Priority:** HIGH
**Estimated Effort:** Small

**Description:**
Create the `internal/domain/` package directory and prepare for type extraction.

**Files Added:**
- `internal/domain/types.go` (new, empty)
- `internal/domain/normalize.go` (new, empty)
- `internal/domain/normalize_test.go` (new, empty)

**Testing:**
- Build succeeds: `go build ./...`
- No functionality changed yet

---

#### TASK-002: Extract shared types to internal/domain/types.go
**Owner:** Ollama (initial) → go-systems-expert (review) → product-requirements-guardian (verify contracts)
**Priority:** HIGH
**Estimated Effort:** Small

**Description:**
Move duplicated type definitions from both `cmd/server/main.go` and `cmd/cli/main.go` to a shared package. Enhance types with additional fields for RDAP support while maintaining backward compatibility.

**Files Modified:**
- `internal/domain/types.go` (implement)
  - Add: `type Domain struct` (new enhanced type)
  - Add: `type Status int` (enum: Unknown, Available, Taken, Error)
  - Add: `type Result struct` (enhanced with Source, CheckedAt, Duration)
  - Add: `func (s Status) String() string`
  - Add: `func (r Result) MarshalJSON() ([]byte, error)` (custom JSON for compatibility)

**Files NOT Modified Yet:**
- `cmd/server/main.go` (will update in TASK-005)
- `cmd/cli/main.go` (will update in TASK-006)

**Testing:**
- Unit test: Type definitions compile
- Unit test: JSON marshaling produces expected output
- Unit test: Status.String() returns correct values
- Build succeeds: `go build ./...`

**Acceptance Criteria:**
- Types are properly documented with Go comments
- JSON tags match existing API contract exactly
- All exported types have comprehensive doc comments

**Reviewer:** product-requirements-guardian (confirm JSON structure matches existing API)

---

#### TASK-003: Implement domain normalization logic
**Owner:** Ollama (initial) → go-systems-expert (review) → qa-test-analyst (test)
**Priority:** HIGH (fixes existing bug)
**Estimated Effort:** Small

**Description:**
Implement the domain normalization function using CLI logic (safer approach that doesn't corrupt non-.com TLDs). This fixes the existing server bug.

**Files Modified:**
- `internal/domain/normalize.go`
  - Add: `var ErrEmptyDomain = errors.New(...)`
  - Add: `var ErrInvalidFormat = errors.New(...)`
  - Add: `func Normalize(input string) (Domain, error)` (CLI logic: add .com only if no dot)
  - Add: `func NormalizeBatch(inputs []string) ([]Domain, []error)`

- `internal/domain/normalize_test.go`
  - Add: `TestNormalize()` with test cases:
    - `"trucore"` → `Domain{Full: "trucore.com", Name: "trucore", TLD: "com"}`
    - `"foo.io"` → `Domain{Full: "foo.io", Name: "foo", TLD: "io"}` (preserves non-.com)
    - `"example.org"` → `Domain{Full: "example.org", Name: "example", TLD: "org"}` (bug fix test)
    - `"TRUCORE"` → `Domain{Full: "trucore.com", ...}` (lowercase)
    - `"  trucore  "` → `Domain{Full: "trucore.com", ...}` (trim whitespace)
    - `""` → `ErrEmptyDomain`
    - `"sub.example.com"` → `Domain{Full: "sub.example.com", Name: "sub.example", TLD: "com"}`
  - Add: `TestNormalizeBatch()`

**Testing:**
- All unit tests pass: `go test ./internal/domain/...`
- Edge case: Empty string returns error
- Edge case: Whitespace-only returns error
- Edge case: Subdomains handled correctly
- Bug fix confirmed: `example.org` does NOT become `example.org.com`

**Acceptance Criteria:**
- Test coverage ≥ 90%
- All edge cases covered
- Clear error messages for invalid input
- Documentation explains the normalization rules

**Reviewer:** qa-test-analyst (validate test coverage and edge cases)

---

### Phase 2: Checker Implementation

#### TASK-004: Implement DNS pre-filter
**Owner:** Ollama (initial) → go-systems-expert (review)
**Priority:** MEDIUM
**Estimated Effort:** Small

**Description:**
Implement fast DNS NS record lookup to pre-filter domain availability. This provides a 10-120ms check before hitting RDAP/WHOIS.

**Files Modified:**
- `internal/checker/dns.go`
  - Add: `type dnsFilter struct` (implements DNSFilter interface)
  - Add: `func NewDNSFilter() DNSFilter`
  - Add: `func NewDNSFilterWithResolver(r *net.Resolver) DNSFilter` (for testing)
  - Add: `func (f *dnsFilter) HasNameservers(ctx context.Context, d domain.Domain) (bool, error)`

- `internal/checker/dns_test.go`
  - Add: `TestDNSFilter_HasNameservers()` with:
    - Known registered domain (google.com) → true
    - Non-existent domain → false
    - Timeout handling
    - NXDOMAIN handling

**Dependencies:**
- Go stdlib: `net`, `context`

**Testing:**
- Unit tests pass: `go test ./internal/checker/ -run TestDNS`
- Integration test with real DNS (google.com)
- Mock test with custom resolver
- Timeout test (500ms deadline)

**Acceptance Criteria:**
- Handles DNS errors gracefully (NXDOMAIN, timeout, SERVFAIL)
- Returns false for NXDOMAIN (domain available)
- Returns true when NS records exist
- Fast execution (< 500ms typical case)

---

#### TASK-005: Implement RDAP client
**Owner:** Ollama (initial) → go-systems-expert (review) → project-research-analyst (verify protocol)
**Priority:** HIGH
**Estimated Effort:** Medium

**Description:**
Implement RDAP protocol client for querying Verisign's RDAP service. This is the primary domain checking method.

**Files Modified:**
- `internal/checker/rdap.go`
  - Add: `var ErrNotFound = errors.New("domain not found")`
  - Add: `var ErrRDAPFailed = errors.New("rdap lookup failed")`
  - Add: `var ErrUnsupportedTLD = errors.New("tld not supported by rdap")`
  - Add: `type RDAPResponse struct` (parsed JSON structure)
  - Add: `type RDAPEvent struct`
  - Add: `type rdapClient struct` (implements RDAPClient interface)
  - Add: `func NewRDAPClient() RDAPClient`
  - Add: `func NewRDAPClientWithHTTP(client *http.Client) RDAPClient` (for testing)
  - Add: `func defaultBootstrap() map[string]string` (TLD → RDAP server mapping)
  - Add: `func (c *rdapClient) Lookup(ctx context.Context, d domain.Domain) (*RDAPResponse, error)`

- `internal/checker/rdap_test.go`
  - Add: `TestRDAPClient_Lookup()` with httptest mock server
  - Add: Test cases for HTTP 200 (domain registered)
  - Add: Test cases for HTTP 404 (domain available)
  - Add: Test cases for HTTP 429 (rate limited)
  - Add: Test cases for timeout
  - Add: Test cases for unsupported TLD

**Dependencies:**
- Go stdlib: `net/http`, `encoding/json`, `context`, `time`

**RDAP Endpoint:**
- Verisign .com: `https://rdap.verisign.com/com/v1/domain/{DOMAIN}`

**Testing:**
- Unit tests with mock HTTP server: `go test ./internal/checker/ -run TestRDAP`
- Mock returns HTTP 404 → result is ErrNotFound
- Mock returns HTTP 200 → result is RDAPResponse with data
- Mock returns HTTP 429 → result is ErrRateLimited
- Mock timeout → returns context deadline error
- Unsupported TLD (e.g., .xyz) → returns ErrUnsupportedTLD

**Acceptance Criteria:**
- Correctly interprets HTTP status codes
- Parses JSON response structure
- Handles network errors gracefully
- 5-second timeout per request
- Bootstrap map supports .com, .net, .org, .io, .dev, .app, .ai

**Reviewer:** project-research-analyst (verify RDAP protocol compliance)

---

#### TASK-006: Implement WHOIS fallback client
**Owner:** Ollama (initial) → go-systems-expert (review)
**Priority:** MEDIUM
**Estimated Effort:** Medium

**Description:**
Implement WHOIS protocol client as fallback when RDAP fails or is unavailable. This maintains reliability when RDAP services are down.

**Files Modified:**
- `internal/checker/whois.go`
  - Add: `type whoisClient struct` (implements WHOISClient interface)
  - Add: `func NewWHOISClient() WHOISClient`
  - Add: `func defaultWHOISServers() map[string]string` (TLD → WHOIS server mapping)
  - Add: `func (c *whoisClient) Query(ctx context.Context, d domain.Domain) (string, error)`
  - Add: `func parseWHOISAvailability(text string) bool` (extract from current server code)

- `internal/checker/whois_test.go`
  - Add: `TestWHOISClient_Query()` with mock TCP server
  - Add: `TestParseWHOISAvailability()` with sample responses:
    - "No match for domain" → true (available)
    - "Domain Name: EXAMPLE.COM" → false (taken)
    - "Registry Domain ID:" → false (taken)

**Dependencies:**
- Go stdlib: `net`, `bufio`, `strings`, `context`

**WHOIS Servers:**
- .com: `whois.verisign-grs.com:43`
- .net: `whois.verisign-grs.com:43`
- .org: `whois.pir.org:43`
- Default fallback: `whois.iana.org:43`

**Testing:**
- Unit tests with mock TCP server: `go test ./internal/checker/ -run TestWHOIS`
- Parse test with real WHOIS response samples
- Timeout test (10-second deadline)
- Connection failure handling

**Acceptance Criteria:**
- Sends proper WHOIS query format (`{DOMAIN}\r\n`)
- Reads full response until EOF
- Parses availability indicators correctly
- 10-second timeout per query
- Graceful handling of connection errors

---

#### TASK-007: Implement main Checker orchestration
**Owner:** Ollama (initial) → go-systems-expert (review)
**Priority:** HIGH
**Estimated Effort:** Medium

**Description:**
Implement the main Checker that orchestrates DNS pre-filter, RDAP lookup, and WHOIS fallback. This is the core business logic.

**Files Modified:**
- `internal/checker/checker.go`
  - Add: `type Config struct` (Timeout, MaxConcurrency, UseDNSFilter, FallbackToWHOIS)
  - Add: `func DefaultConfig() Config`
  - Add: `type checker struct` (private implementation)
  - Add: `func New(cfg Config, rdap RDAPClient, whois WHOISClient, dns DNSFilter) Checker`
  - Add: `func (c *checker) Check(ctx context.Context, domainName string) domain.Result`
  - Add: `func (c *checker) CheckBatch(ctx context.Context, domains []string) []domain.Result`
  - Add: `func (c *checker) checkRDAP(ctx context.Context, d domain.Domain) domain.Result`
  - Add: `func (c *checker) checkWHOIS(ctx context.Context, d domain.Domain) domain.Result`

- `internal/checker/checker_test.go`
  - Add: `type mockRDAPClient struct` (test mock)
  - Add: `type mockWHOISClient struct` (test mock)
  - Add: `type mockDNSFilter struct` (test mock)
  - Add: `TestChecker_Check()` with scenarios:
    - Available via RDAP
    - Taken via RDAP
    - RDAP fails, WHOIS fallback shows available
    - Bare name normalized to .com
    - DNS pre-filter optimization
  - Add: `TestChecker_CheckBatch()` (concurrent checking)

**Flow Logic:**
1. Normalize domain name
2. If DNS filter enabled: check NS records (fast path)
3. Query RDAP (primary method)
4. If RDAP fails and fallback enabled: query WHOIS
5. Return result with source, timing, error details

**Testing:**
- Unit tests with all mocks: `go test ./internal/checker/ -run TestChecker`
- Test DNS → RDAP → WHOIS cascade
- Test concurrent batch processing (10 parallel)
- Test timeout enforcement
- Test error handling at each stage

**Acceptance Criteria:**
- Check() returns complete Result with timing
- CheckBatch() processes concurrently with semaphore
- Respects MaxConcurrency limit (10 default)
- Timeout applied per domain (15s default)
- Source field indicates method used (rdap/whois/dns)

---

### Phase 3: Server Integration

#### TASK-008: Create HTTP handlers in internal/server
**Owner:** Ollama (initial) → go-systems-expert (review) → product-requirements-guardian (verify API)
**Priority:** HIGH
**Estimated Effort:** Medium

**Description:**
Extract HTTP handler logic from `cmd/server/main.go` into a separate `internal/server` package with proper separation of concerns.

**Files Modified:**
- `internal/server/handlers.go`
  - Add: `const MaxDomainsPerRequest = 100`
  - Add: `type CheckRequest struct` (moved from server main)
  - Add: `type CheckResponse struct` (moved from server main)
  - Add: `type Handler struct` (holds checker dependency)
  - Add: `func NewHandler(c checker.Checker) *Handler`
  - Add: `func (h *Handler) CheckDomains(w http.ResponseWriter, r *http.Request)` (POST /check)
  - Add: `func (h *Handler) CheckSingleDomain(w http.ResponseWriter, r *http.Request)` (GET /check/{domain})
  - Add: `func (h *Handler) Health(w http.ResponseWriter, r *http.Request)` (GET /health)

- `internal/server/handlers_test.go`
  - Add: `type mockChecker struct` (implements checker.Checker)
  - Add: `TestHandler_CheckDomains()` (POST /check)
  - Add: `TestHandler_CheckDomains_MethodNotAllowed()` (GET returns 405)
  - Add: `TestHandler_CheckDomains_InvalidJSON()` (400 error)
  - Add: `TestHandler_CheckDomains_NoDomains()` (400 error)
  - Add: `TestHandler_CheckDomains_TooManyDomains()` (>100 returns 400)
  - Add: `TestHandler_CheckSingleDomain()`
  - Add: `TestHandler_Health()`

**Dependencies:**
- Go stdlib: `net/http`, `encoding/json`
- Internal: `domaincheck/internal/checker`, `domaincheck/internal/domain`

**Testing:**
- Unit tests with httptest: `go test ./internal/server/...`
- Test all HTTP status codes (200, 400, 405)
- Test JSON parsing and encoding
- Test domain count limits
- Test error responses

**Acceptance Criteria:**
- All endpoint paths match existing API (`/check`, `/check/{domain}`, `/health`)
- All HTTP methods match existing API (POST for /check, GET for others)
- JSON response structure matches existing format exactly
- Error messages are descriptive
- Handlers are stateless (only dependency is Checker)

**Reviewer:** product-requirements-guardian (verify API contract compliance)

---

#### TASK-009: Refactor cmd/server/main.go
**Owner:** Ollama (initial) → go-systems-expert (review)
**Priority:** HIGH
**Estimated Effort:** Small

**Description:**
Simplify server entry point to just wire up dependencies and start HTTP server. Remove all business logic and interactive stdin mode.

**Files Modified:**
- `cmd/server/main.go`
  - Delete: Lines 23-39 (type definitions) → moved to internal/domain
  - Delete: Lines 41-115 (`checkDomain()` function) → moved to internal/checker
  - Delete: Lines 117-175 (handler implementations) → moved to internal/server
  - Delete: Lines 211-231 (interactive stdin goroutine) → removed per requirements
  - Keep: Environment variable reading (`PORT`)
  - Keep: Logging statements
  - Update: Use `checker.New()` to create checker
  - Update: Use `server.NewHandler()` to create handlers
  - Update: Register routes with new handlers

**New Implementation:**
```go
package main

import (
    "log"
    "net/http"
    "os"

    "domaincheck/internal/checker"
    "domaincheck/internal/server"
)

func main() {
    port := os.Getenv("PORT")
    if port == "" {
        port = "8765"
    }

    cfg := checker.DefaultConfig()
    c := checker.New(cfg, nil, nil, nil) // Use defaults
    h := server.NewHandler(c)

    http.HandleFunc("/check", h.CheckDomains)
    http.HandleFunc("/check/", h.CheckSingleDomain)
    http.HandleFunc("/health", h.Health)

    log.Printf("Domain checker service starting on port %s", port)
    log.Printf("Endpoints:")
    log.Printf("  POST /check          - Check multiple domains")
    log.Printf("  GET  /check/{domain} - Check single domain")
    log.Printf("  GET  /health         - Health check")

    if err := http.ListenAndServe(":"+port, nil); err != nil {
        log.Fatalf("Server failed: %v", err)
    }
}
```

**Files Modified:**
- `cmd/server/main.go` (reduce from ~237 lines to ~35 lines)

**Testing:**
- Build succeeds: `go build ./cmd/server`
- Server starts: `./cmd/server/server`
- API endpoints work: `curl http://localhost:8765/health`
- POST /check works: `curl -X POST -d '{"domains":["trucore.com"]}' http://localhost:8765/check`
- GET /check/{domain} works: `curl http://localhost:8765/check/trucore.com`

**Acceptance Criteria:**
- Server starts on port 8765 (or PORT env var)
- All API endpoints return expected responses
- Logging output matches existing format
- No interactive stdin mode
- File reduced from 237 lines to ~35 lines

---

### Phase 4: CLI Integration

#### TASK-010: Refactor cmd/cli/main.go
**Owner:** Ollama (initial) → go-systems-expert (review) → ui-ux-design-specialist (verify UX)
**Priority:** HIGH
**Estimated Effort:** Medium

**Description:**
Refactor CLI to use the new checker package directly (optional local mode) or communicate with server (existing mode). Update to use shared types and normalization.

**Files Modified:**
- `cmd/cli/main.go`
  - Delete: Lines 16-32 (type definitions) → moved to internal/domain
  - Delete: Lines 145-151 (normalization) → moved to internal/domain
  - Update: Import `domaincheck/internal/checker` and `domaincheck/internal/domain`
  - Add: Option to use local checker (bypass server) with `--local` flag
  - Update: `parseDomainList()` to use domain.Normalize()
  - Update: Output formatting to use domain.Result type
  - Keep: All existing flags (-s, -f, -j, -a, -q, -h)
  - Keep: Exit code behavior
  - Keep: Output format (symbols ✓✗?, alignment)

**New Features (Optional):**
- Add: `--local` flag to use checker directly without server
- Add: `-v` verbose flag for enhanced output with timing/method

**Testing:**
- Build succeeds: `go build ./cmd/cli`
- All existing flags work: `-s`, `-f`, `-j`, `-a`, `-q`, `-h`
- File input works: `./cli -f domains.txt`
- Stdin works: `echo "trucore" | ./cli`
- JSON output works: `./cli -j trucore`
- Quiet mode works: `./cli -q trucore && echo "available"`
- Local mode works: `./cli --local trucore` (bypasses server)

**Acceptance Criteria:**
- All existing CLI behaviors unchanged
- Output format matches existing (backward compatible)
- Exit codes unchanged (0=available, 1=taken/error)
- File reduced from ~236 lines to ~180 lines
- Optional local mode works without server running

**Reviewer:** ui-ux-design-specialist (verify CLI output format)

---

### Phase 5: Configuration & Documentation

#### TASK-011: Create shared configuration package
**Owner:** Ollama (initial) → go-systems-expert (review)
**Priority:** LOW
**Estimated Effort:** Small

**Description:**
Extract shared constants and configuration into a dedicated package for consistency.

**Files Modified:**
- `internal/config/config.go`
  - Add: `const DefaultPort = "8765"`
  - Add: `const DefaultServerURL = "http://localhost:8765"`
  - Add: `const MaxDomainsPerRequest = 100`
  - Add: `const DefaultTimeout = 15 * time.Second`
  - Add: `const DefaultMaxConcurrency = 10`
  - Add: `const WHOISTimeout = 10 * time.Second`
  - Add: `const RDAPTimeout = 5 * time.Second`
  - Add: `const DNSTimeout = 500 * time.Millisecond`

**Testing:**
- Build succeeds: `go build ./...`
- Constants accessible from other packages

**Acceptance Criteria:**
- All magic numbers replaced with named constants
- Clear documentation for each constant
- Grouped logically (server, client, checker, timeouts)

---

#### TASK-012: Update go.mod with dependencies
**Owner:** go-systems-expert
**Priority:** MEDIUM
**Estimated Effort:** Small

**Description:**
Update go.mod to include any new external dependencies and set proper module path.

**Files Modified:**
- `go.mod`
  - Ensure module path: `module domaincheck`
  - Add dependency: `require github.com/likexian/whois v1.15.1` (if using for WHOIS)
  - Or note: "No external dependencies" if using raw net.Dial for WHOIS
  - Set `go 1.21` (or higher)

**Commands:**
```bash
go mod tidy
go mod verify
```

**Testing:**
- `go mod tidy` completes successfully
- `go build ./...` succeeds
- `go test ./...` succeeds

**Acceptance Criteria:**
- Module path is correct
- All dependencies resolved
- go.sum is up to date

---

#### TASK-013: Update README.md
**Owner:** infrastructure-docs-updater → product-requirements-guardian (review)
**Priority:** MEDIUM
**Estimated Effort:** Medium

**Description:**
Update project documentation to reflect new architecture, RDAP upgrade, and removed features.

**Files Modified:**
- `README.md`
  - Update: Architecture section (mention internal/ packages)
  - Update: Features section (mention RDAP, DNS pre-filter)
  - Add: Migration section (v1.x to v2.0)
  - Update: Installation section (if build process changed)
  - Update: Usage examples (show new flags if added)
  - Remove: References to interactive server mode
  - Add: Performance notes (RDAP 3-5x faster than WHOIS)
  - Add: Troubleshooting section (RDAP errors, rate limits)

**New Sections:**
- "What's New in v2.0"
- "Migration Guide"
- "Architecture Overview"
- "Contributing"

**Testing:**
- Markdown renders correctly
- All code examples work
- Links are valid
- Examples tested manually

**Acceptance Criteria:**
- Clear explanation of RDAP upgrade
- Migration path documented
- Examples updated and verified
- No references to removed features

**Reviewer:** product-requirements-guardian (verify accuracy)

---

#### TASK-014: Create MIGRATION.md
**Owner:** infrastructure-docs-updater → ui-ux-design-specialist (review)
**Priority:** MEDIUM
**Estimated Effort:** Small

**Description:**
Create detailed migration guide for users upgrading from v1.x to v2.0.

**Files Added:**
- `MIGRATION.md`
  - Section: Breaking Changes
    - Interactive server mode removed
    - Error message format changed (lowercase → uppercase)
    - WHOIS → RDAP (different error types)
  - Section: New Features
    - Faster checks via RDAP
    - Performance timing in responses
    - Check method transparency
  - Section: Backward Compatibility
    - API endpoints unchanged
    - CLI flags unchanged
    - JSON structure extended (additive only)
  - Section: Upgrade Checklist
    - Update error parsing
    - Remove telnet/stdin integrations
    - Test error handling

**Testing:**
- Markdown renders correctly
- Migration steps verified
- Examples tested

**Acceptance Criteria:**
- All breaking changes documented
- Workarounds provided where applicable
- Clear upgrade path
- Examples of before/after

**Reviewer:** ui-ux-design-specialist (verify UX messaging)

---

### Phase 6: Testing & Quality Assurance

#### TASK-015: Create integration test suite
**Owner:** qa-test-analyst
**Priority:** HIGH
**Estimated Effort:** Medium

**Description:**
Create comprehensive integration tests that verify end-to-end functionality matches the old behavior exactly.

**Files Added:**
- `test/integration_test.go`
  - Test: Server API endpoints (POST /check, GET /check/{domain}, GET /health)
  - Test: CLI with various flags (-j, -a, -q, -f)
  - Test: Domain normalization (bare names, TLDs, subdomains)
  - Test: Error handling (timeouts, invalid domains, rate limits)
  - Test: Concurrency (100 domains at once)
  - Test: RDAP → WHOIS fallback
  - Test: DNS pre-filter optimization

**Testing Strategy:**
- Use real network calls (RDAP, DNS)
- Mock WHOIS with test server
- Compare output with v1.x baseline
- Verify JSON structure byte-for-byte (excluding timing fields)

**Commands:**
```bash
go test ./test/... -v
go test ./test/... -race
```

**Acceptance Criteria:**
- All integration tests pass
- No race conditions detected
- Output matches v1.x baseline
- Error messages are appropriate

---

#### TASK-016: Validate backward compatibility
**Owner:** qa-test-analyst → product-requirements-guardian (review)
**Priority:** CRITICAL
**Estimated Effort:** Medium

**Description:**
Systematic validation that all user-facing contracts remain unchanged.

**Test Cases:**

**API Endpoint Testing:**
- [ ] `POST /check` accepts CheckRequest JSON
- [ ] `POST /check` returns CheckResponse JSON with exact field names
- [ ] `GET /check/{domain}` works with path parameter
- [ ] `GET /health` returns `{"status": "ok"}`
- [ ] Invalid JSON returns HTTP 400
- [ ] >100 domains returns HTTP 400
- [ ] GET to /check returns HTTP 405

**CLI Testing:**
- [ ] `domaincheck trucore` works
- [ ] `domaincheck -f file.txt` works
- [ ] `echo trucore | domaincheck -` works
- [ ] `domaincheck -j trucore` outputs valid JSON
- [ ] `domaincheck -a trucore taken.com` shows only available
- [ ] `domaincheck -q trucore` exits with code 0/1
- [ ] `domaincheck -h` shows help

**Domain Normalization:**
- [ ] `trucore` → `trucore.com` (add .com)
- [ ] `trucore.com` → `trucore.com` (unchanged)
- [ ] `example.org` → `example.org` (preserve non-.com) **[BUG FIX]**
- [ ] `TRUCORE` → `trucore.com` (lowercase)
- [ ] `  trucore  ` → `trucore.com` (trim)

**JSON Structure:**
- [ ] Field names unchanged: `domain`, `available`, `error`, `results`, `checked`, `available`, `taken`, `errors`
- [ ] Field types unchanged: string, bool, int, array
- [ ] New fields are optional (don't break old parsers)

**Acceptance Criteria:**
- 100% of backward compatibility tests pass
- No breaking changes detected
- Baseline comparison shows identical behavior (except bug fixes)

**Reviewer:** product-requirements-guardian (sign off on compatibility)

---

#### TASK-017: Performance benchmarking
**Owner:** qa-test-analyst
**Priority:** LOW
**Estimated Effort:** Small

**Description:**
Create benchmarks to validate RDAP performance improvements over WHOIS.

**Files Added:**
- `test/benchmark_test.go`
  - Benchmark: `BenchmarkCheckDomain_RDAP()`
  - Benchmark: `BenchmarkCheckDomain_WHOIS()`
  - Benchmark: `BenchmarkCheckDomain_DNS()`
  - Benchmark: `BenchmarkCheckBatch_10()`
  - Benchmark: `BenchmarkCheckBatch_100()`

**Commands:**
```bash
go test -bench=. ./test/...
go test -bench=. -benchmem ./test/...
```

**Expected Results:**
- RDAP: 100-500ms per domain
- WHOIS: 200-2000ms per domain
- DNS: 10-120ms per domain
- RDAP should be 3-5x faster than WHOIS

**Acceptance Criteria:**
- Benchmarks complete successfully
- Performance improvements documented
- Memory usage is reasonable

---

### Phase 7: Deployment & Rollout

#### TASK-018: Update Makefile
**Owner:** go-systems-expert
**Priority:** LOW
**Estimated Effort:** Small

**Description:**
Update build targets and add new commands for testing, linting, and building.

**Files Modified:**
- `Makefile`
  - Update: `build` target (build both server and CLI)
  - Add: `test` target (run all tests)
  - Add: `integration-test` target (run integration tests)
  - Add: `benchmark` target (run benchmarks)
  - Add: `lint` target (run go vet, golangci-lint if available)
  - Add: `coverage` target (generate coverage report)
  - Update: `clean` target (remove binaries)
  - Add: `install` target (install binaries to $GOPATH/bin)

**Example Targets:**
```makefile
.PHONY: build test integration-test benchmark lint coverage clean install

build:
	go build -o bin/server ./cmd/server
	go build -o bin/cli ./cmd/cli

test:
	go test ./... -v

integration-test:
	go test ./test/... -v

benchmark:
	go test -bench=. ./test/...

lint:
	go vet ./...
	golangci-lint run || true

coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out

clean:
	rm -rf bin/
	rm -f coverage.out

install:
	go install ./cmd/server
	go install ./cmd/cli
```

**Testing:**
- `make build` succeeds
- `make test` runs all tests
- `make clean` removes artifacts

**Acceptance Criteria:**
- All targets work
- Clear target descriptions
- Binaries output to bin/ directory

---

#### TASK-019: Create release notes
**Owner:** product-requirements-guardian → infrastructure-docs-updater (edit)
**Priority:** MEDIUM
**Estimated Effort:** Small

**Description:**
Prepare release notes for v2.0 documenting all changes, improvements, and migration requirements.

**Files Added:**
- `CHANGELOG.md` (or section in README)
  - Version: 2.0.0
  - Date: TBD
  - Breaking Changes:
    - Interactive server stdin mode removed
    - Error messages now uppercase (TIMEOUT vs "timeout")
  - New Features:
    - RDAP protocol support (3-5x faster)
    - DNS NS pre-filter for instant availability checks
    - Performance timing in API responses
    - Check method transparency (know if RDAP or WHOIS was used)
  - Bug Fixes:
    - Fixed domain normalization bug that corrupted non-.com TLDs
  - Internal Improvements:
    - Eliminated code duplication (~100 lines removed)
    - Better testability with interface-based design
    - Proper package structure (internal/)
  - Deprecations:
    - WHOIS is now fallback only (RDAP primary)

**Testing:**
- Markdown renders correctly
- All links work
- Version number correct

**Acceptance Criteria:**
- Complete list of changes
- Migration guide referenced
- Known issues documented (if any)

---

#### TASK-020: Final validation and sign-off
**Owner:** product-requirements-guardian → All reviewers
**Priority:** CRITICAL
**Estimated Effort:** Medium

**Description:**
Final validation checklist before considering refactoring complete.

**Validation Checklist:**

**Functional Requirements:**
- [ ] All API endpoints work identically to v1.x
- [ ] All CLI flags work identically to v1.x
- [ ] Domain checking produces same results
- [ ] Error handling is comprehensive
- [ ] Concurrency limits enforced (10 parallel)
- [ ] Timeout behavior unchanged (10-15s)

**Code Quality:**
- [ ] No code duplication remains
- [ ] All packages properly documented
- [ ] Test coverage ≥ 80%
- [ ] No race conditions (go test -race passes)
- [ ] No linter warnings
- [ ] All TODOs resolved or documented

**Documentation:**
- [ ] README.md updated and accurate
- [ ] MIGRATION.md complete
- [ ] CHANGELOG.md comprehensive
- [ ] Code comments explain "why" not "what"
- [ ] API documentation matches implementation

**Performance:**
- [ ] RDAP faster than old WHOIS implementation
- [ ] DNS pre-filter provides measurable improvement
- [ ] No memory leaks detected
- [ ] Benchmarks pass

**Backward Compatibility:**
- [ ] JSON structure unchanged (additive only)
- [ ] CLI flags unchanged
- [ ] Exit codes unchanged
- [ ] Error messages structured but familiar

**Acceptance Criteria:**
- All checklist items marked complete
- Sign-off from:
  - product-requirements-guardian (requirements met)
  - go-systems-expert (code quality)
  - qa-test-analyst (testing complete)
  - ui-ux-design-specialist (UX preserved)
  - infrastructure-docs-updater (docs complete)

---

## Risk Assessment & Mitigation

### Critical Risks

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| Breaking API compatibility | LOW | CRITICAL | Comprehensive integration tests, JSON comparison |
| RDAP rate limiting | MEDIUM | HIGH | WHOIS fallback, exponential backoff, DNS pre-filter |
| Domain normalization regression | LOW | MEDIUM | Extensive test suite, edge case coverage |
| Performance degradation | LOW | MEDIUM | Benchmarks, profiling, load testing |
| RDAP service downtime | MEDIUM | MEDIUM | WHOIS fallback, retry logic, circuit breaker |

### Medium Risks

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| Test coverage gaps | MEDIUM | MEDIUM | Code review, coverage reports (≥80%) |
| Documentation outdated | MEDIUM | LOW | Review by infrastructure-docs-updater |
| Migration difficulties | LOW | MEDIUM | Detailed migration guide, examples |

### Mitigation Strategies

1. **Rollback Plan:**
   - Tag v1.x as `v1.0-stable` before refactoring
   - Maintain v1.x branch for critical bug fixes
   - Document exact rollback procedure

2. **Gradual Rollout:**
   - Deploy to staging environment first
   - Monitor error rates and latency
   - Canary deployment (10% → 50% → 100%)

3. **Feature Flags:**
   - Environment variable to disable RDAP (force WHOIS)
   - Environment variable to disable DNS pre-filter
   - Allows quick mitigation of issues in production

---

## Success Metrics

### Quantitative Metrics

| Metric | Current (v1.x) | Target (v2.0) | Measurement |
|--------|----------------|---------------|-------------|
| Code duplication | ~100 lines (21%) | 0 lines (0%) | Static analysis |
| Average check latency | 500-2000ms | 100-500ms | Benchmarks |
| Test coverage | Unknown | ≥80% | go test -cover |
| Lines of code (total) | ~473 | ~600-700 | cloc |
| Lines of code (main.go) | 473 | ~250 | Split across packages |
| Build time | <1s | <2s | time go build |

### Qualitative Metrics

- [ ] All stakeholders approve the plan
- [ ] Code is more maintainable (subjective, team review)
- [ ] New developers can understand structure faster
- [ ] Testing is easier with mocks
- [ ] Future TLD support easier to add

---

## Timeline Estimate

| Phase | Tasks | Estimated Effort | Dependencies |
|-------|-------|-----------------|--------------|
| Phase 1 | TASK-001 to TASK-003 | 2-3 days | None |
| Phase 2 | TASK-004 to TASK-007 | 4-5 days | Phase 1 complete |
| Phase 3 | TASK-008 to TASK-009 | 2-3 days | Phase 2 complete |
| Phase 4 | TASK-010 | 1-2 days | Phase 3 complete |
| Phase 5 | TASK-011 to TASK-014 | 2-3 days | Phases 1-4 complete |
| Phase 6 | TASK-015 to TASK-017 | 3-4 days | All implementation complete |
| Phase 7 | TASK-018 to TASK-020 | 1-2 days | Phase 6 complete |

**Total Estimate:** 15-22 working days (3-4.5 weeks)

**Note:** This assumes:
- Ollama handles initial code generation (60-70% of coding work)
- Human reviewers focus on review and refinement
- No major blockers or scope changes

---

## Approval & Sign-Off

### Stakeholder Approval Required

- [ ] **product-requirements-guardian** - Requirements complete and accurate
- [ ] **project-research-analyst** - Technical approach feasible and optimal
- [ ] **go-systems-expert** - Architecture sound and idiomatic
- [ ] **ui-ux-design-specialist** - UX impact acceptable
- [ ] **qa-test-analyst** - Testing strategy comprehensive
- [ ] **infrastructure-docs-updater** - Documentation plan complete

### Questions for Final Approval

Before proceeding with implementation, please confirm:

1. **RDAP Adoption:** Are you comfortable switching from WHOIS to RDAP as the primary method, with WHOIS as fallback?

2. **Breaking Changes:** The only breaking change is removal of interactive server stdin mode. Is this acceptable?

3. **Timeline:** The estimated 3-4.5 weeks timeline assumes Ollama does initial coding. Does this align with your schedule?

4. **External Dependencies:** We may add `github.com/likexian/whois` for WHOIS fallback. Is adding an external dependency acceptable, or should we use raw TCP sockets?

5. **Testing Scope:** Integration tests will make real network calls to RDAP servers. Is this acceptable, or should we mock all external calls?

6. **Documentation Priority:** Should documentation updates (TASK-013, TASK-014) happen before or after implementation?

---

## Next Steps

Once this plan is approved:

1. **Create Git Branch:** `git checkout -b refactor/eliminate-duplication`
2. **Begin Phase 1:** Execute TASK-001 (create directory structure)
3. **Daily Progress Updates:** Track completion of tasks
4. **Code Reviews:** Each task reviewed by assigned specialist
5. **Continuous Integration:** Tests run on every commit
6. **Weekly Demos:** Show progress to stakeholders

---

## Appendix A: File Change Summary

### Files to be Created (17 new files)

| File | Purpose | Lines (est.) |
|------|---------|--------------|
| `internal/domain/types.go` | Shared type definitions | ~80 |
| `internal/domain/normalize.go` | Domain normalization logic | ~60 |
| `internal/domain/normalize_test.go` | Normalization tests | ~120 |
| `internal/checker/checker.go` | Main checker orchestration | ~150 |
| `internal/checker/checker_test.go` | Checker tests with mocks | ~200 |
| `internal/checker/rdap.go` | RDAP client | ~120 |
| `internal/checker/rdap_test.go` | RDAP tests | ~150 |
| `internal/checker/whois.go` | WHOIS fallback | ~100 |
| `internal/checker/whois_test.go` | WHOIS tests | ~80 |
| `internal/checker/dns.go` | DNS pre-filter | ~60 |
| `internal/checker/dns_test.go` | DNS tests | ~80 |
| `internal/server/handlers.go` | HTTP handlers | ~120 |
| `internal/server/handlers_test.go` | Handler tests | ~150 |
| `internal/config/config.go` | Shared configuration | ~40 |
| `test/integration_test.go` | Integration tests | ~200 |
| `test/benchmark_test.go` | Benchmarks | ~80 |
| `MIGRATION.md` | Migration guide | Documentation |

**Total new code:** ~1,790 lines

### Files to be Modified (4 files)

| File | Current Lines | New Lines (est.) | Change |
|------|---------------|------------------|--------|
| `cmd/server/main.go` | 237 | ~35 | -202 lines (85% reduction) |
| `cmd/cli/main.go` | 236 | ~180 | -56 lines (24% reduction) |
| `README.md` | ~100 | ~200 | +100 lines (expanded) |
| `go.mod` | 3 | ~8 | +5 lines (dependencies) |

**Total line reduction in entry points:** -258 lines

### Net Change

- **Old codebase:** ~473 lines (2 files)
- **New codebase:** ~1,790 new + ~415 refactored = **~2,205 lines** (31 files)
- **Net increase:** ~1,732 lines (but with 17 test files)
- **Production code only:** ~1,215 lines (without tests)

**Conclusion:** Code size increases due to:
- Proper package structure
- Comprehensive testing (9 test files)
- RDAP/DNS/WHOIS implementations
- Better documentation

But **production code duplication goes from 21% to 0%** and maintainability improves significantly.

---

## Appendix B: External Dependencies

### Option 1: Minimal Dependencies (Recommended)

```go
module domaincheck

go 1.21

// No external dependencies - use only Go stdlib
```

**Pros:**
- No dependency management complexity
- Faster builds
- No security vulnerabilities from third-party code

**Cons:**
- More code to write (raw TCP for WHOIS, raw HTTP for RDAP)
- May need to implement RDAP bootstrap logic

### Option 2: Community Libraries

```go
module domaincheck

go 1.21

require (
    github.com/likexian/whois v1.15.1
    github.com/openrdap/rdap v0.9.1
)
```

**Pros:**
- Battle-tested implementations
- Less code to maintain
- RDAP bootstrap handled automatically

**Cons:**
- External dependencies to track
- Potential security updates needed
- Larger binary size

**Recommendation:** Start with Option 1 (stdlib only), add libraries only if complexity becomes unmanageable.

---

## Appendix C: Glossary

| Term | Definition |
|------|------------|
| **RDAP** | Registration Data Access Protocol - Modern replacement for WHOIS |
| **WHOIS** | Legacy protocol for querying domain registration data |
| **DNS NS** | Domain Name System Name Server records - indicates domain delegation |
| **TLD** | Top-Level Domain (.com, .org, .net, etc.) |
| **Normalization** | Converting user input to standard format (lowercase, add .com, etc.) |
| **Backward Compatibility** | New version works with old clients/scripts |
| **Additive Versioning** | Add new fields without removing old ones |
| **Semaphore** | Concurrency control mechanism (limit parallel operations) |
| **Mock** | Test double that simulates real object behavior |
| **Integration Test** | Test that verifies multiple components work together |
| **Unit Test** | Test that verifies a single function/component in isolation |

---

**END OF IMPLEMENTATION PLAN**

*This plan is subject to approval by all stakeholders. Once approved, implementation will proceed in phases with continuous validation and testing.*
