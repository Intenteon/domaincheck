// Package server provides HTTP handlers for the domain checking API.
package server

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"sync"
	"time"
)

// dashboardHTML embeds the dashboard template for single-binary deployment.
// The embed directive must reference the path relative to this Go source file.
//
//go:embed templates/dashboard.html
var dashboardHTML embed.FS

const (
	// csrfTokenLength is the byte length of generated CSRF tokens (32 bytes = 64 hex chars)
	csrfTokenLength = 32

	// csrfTokenExpiry is how long a CSRF token remains valid
	csrfTokenExpiry = 1 * time.Hour

	// csrfCleanupInterval is how often expired tokens are purged
	csrfCleanupInterval = 10 * time.Minute

	// maxCSRFTokens limits total active tokens to prevent memory exhaustion
	maxCSRFTokens = 10000
)

// csrfToken represents a CSRF token with its expiration time.
type csrfToken struct {
	token     string
	expiresAt time.Time
}

// csrfStore holds active CSRF tokens with thread-safe access.
// Tokens are generated per-session and validated on form submissions.
var csrfStore = struct {
	sync.RWMutex
	tokens map[string]csrfToken
}{
	tokens: make(map[string]csrfToken),
}

// init starts the background cleanup goroutine for expired CSRF tokens.
func init() {
	go cleanupExpiredTokens()
}

// cleanupExpiredTokens periodically removes expired CSRF tokens from the store.
// This prevents memory growth from abandoned sessions.
func cleanupExpiredTokens() {
	ticker := time.NewTicker(csrfCleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		csrfStore.Lock()
		for tokenStr, token := range csrfStore.tokens {
			if now.After(token.expiresAt) {
				delete(csrfStore.tokens, tokenStr)
			}
		}
		csrfStore.Unlock()
	}
}

// generateCSRFToken creates a new cryptographically secure CSRF token
// and stores it with an expiration time.
//
// Returns an error if the token limit (maxCSRFTokens) is reached to prevent
// memory exhaustion attacks.
func generateCSRFToken() (string, error) {
	// SECURITY: Check token count before generating to prevent memory exhaustion
	csrfStore.RLock()
	tokenCount := len(csrfStore.tokens)
	csrfStore.RUnlock()

	if tokenCount >= maxCSRFTokens {
		return "", fmt.Errorf("CSRF token limit exceeded (%d active tokens)", maxCSRFTokens)
	}

	bytes := make([]byte, csrfTokenLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	tokenStr := hex.EncodeToString(bytes)
	expiresAt := time.Now().Add(csrfTokenExpiry)

	csrfStore.Lock()
	csrfStore.tokens[tokenStr] = csrfToken{
		token:     tokenStr,
		expiresAt: expiresAt,
	}
	csrfStore.Unlock()

	return tokenStr, nil
}

// ValidateCSRFToken checks if a CSRF token exists and has not expired.
// This function is exported for use by CheckDomainsHandler to validate
// form submissions from the dashboard.
//
// Returns true if the token is valid, false if missing, expired, or invalid.
//
// SECURITY: Uses a single write lock to prevent TOCTOU race conditions
// between checking expiration and deleting expired tokens.
func ValidateCSRFToken(token string) bool {
	if token == "" {
		return false
	}

	// SECURITY: Use write lock for entire validation to prevent TOCTOU race
	csrfStore.Lock()
	defer csrfStore.Unlock()

	storedToken, exists := csrfStore.tokens[token]
	if !exists {
		return false
	}

	// Check expiration and delete atomically
	if time.Now().After(storedToken.expiresAt) {
		delete(csrfStore.tokens, token)
		return false
	}

	return true
}

// dashboardData holds the template data for rendering the dashboard.
type dashboardData struct {
	CSRFToken string
	BaseURL   string
}

// configuredBaseURL holds the base URL set via SetBaseURL. When empty,
// baseURLForRequest falls back to deriving the URL from request headers.
var configuredBaseURL string

// SetBaseURL configures the base URL used in dashboard API examples.
// This should be called once at startup from the BASE_URL environment variable.
// When set, this value is used verbatim, avoiding Host header injection risks.
func SetBaseURL(url string) {
	configuredBaseURL = url
}

// baseURLForRequest returns the base URL for API examples shown on the dashboard.
// It prefers the configured BASE_URL (set via SetBaseURL) for security. If not
// configured, it derives the URL from request headers with scheme validation.
func baseURLForRequest(r *http.Request) string {
	if configuredBaseURL != "" {
		return configuredBaseURL
	}
	// Fallback: derive from request. Only accept "https" as a forwarded proto;
	// any other value (including injection attempts) defaults to "http".
	scheme := "http"
	if r.Header.Get("X-Forwarded-Proto") == "https" || r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

// DashboardHandler handles GET / for the web-based domain checker dashboard.
//
// The dashboard provides:
//   - API documentation and examples
//   - Interactive form for checking domain availability
//   - Real-time results display
//
// Security:
//   - Generates a unique CSRF token per page load
//   - Sets Content-Security-Policy header to prevent XSS
//   - CSP 'unsafe-inline' is required because styles and scripts are embedded
//     in the HTML template for single-file deployment simplicity. The inline
//     content is static and controlled by the server, not user-generated.
//
// Response: HTML page with embedded styles and JavaScript
func DashboardHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Only serve dashboard at root path
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// Parse the embedded template
	tmpl, err := template.ParseFS(dashboardHTML, "templates/dashboard.html")
	if err != nil {
		log.Printf("Failed to parse dashboard template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Generate CSRF token for this session
	csrfToken, err := generateCSRFToken()
	if err != nil {
		log.Printf("Failed to generate CSRF token: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Prepare template data
	data := dashboardData{
		CSRFToken: csrfToken,
		BaseURL:   baseURLForRequest(r),
	}

	// Set security headers
	// Content-Security-Policy: 'unsafe-inline' is required because the dashboard
	// template embeds styles and scripts directly in the HTML for simplicity.
	// This is acceptable because:
	// 1. The inline content is static and server-controlled (not user-generated)
	// 2. It eliminates the need for separate static file serving
	// 3. The form uses CSRF tokens for request validation
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'unsafe-inline'; style-src 'unsafe-inline'")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Execute template and write response
	if err := tmpl.Execute(w, data); err != nil {
		// Headers already sent, can only log the error
		log.Printf("Failed to execute dashboard template: %v", err)
	}
}
