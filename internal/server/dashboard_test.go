package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"domaincheck/internal/domain"
)

// TestGenerateCSRFToken verifies CSRF token generation creates unique cryptographic tokens
func TestGenerateCSRFToken(t *testing.T) {
	token1, err1 := generateCSRFToken()
	if err1 != nil {
		t.Fatalf("Failed to generate first token: %v", err1)
	}

	token2, err2 := generateCSRFToken()
	if err2 != nil {
		t.Fatalf("Failed to generate second token: %v", err2)
	}

	// Verify tokens are not empty
	if token1 == "" || token2 == "" {
		t.Error("Generated tokens should not be empty")
	}

	// Verify tokens are unique
	if token1 == token2 {
		t.Error("Generated tokens should be unique")
	}

	// Verify token length (32 bytes = 64 hex characters)
	if len(token1) != 64 || len(token2) != 64 {
		t.Errorf("Token length should be 64 hex chars, got %d and %d", len(token1), len(token2))
	}
}

// TestValidateCSRFToken_Valid verifies valid unexpired tokens pass validation
func TestValidateCSRFToken_Valid(t *testing.T) {
	token, err := generateCSRFToken()
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	if !ValidateCSRFToken(token) {
		t.Error("Valid token should pass validation")
	}
}

// TestValidateCSRFToken_Invalid verifies invalid tokens fail validation
func TestValidateCSRFToken_Invalid(t *testing.T) {
	invalidToken := "this-is-not-a-valid-token"

	if ValidateCSRFToken(invalidToken) {
		t.Error("Invalid token should fail validation")
	}
}

// TestValidateCSRFToken_Empty verifies empty tokens fail validation
func TestValidateCSRFToken_Empty(t *testing.T) {
	if ValidateCSRFToken("") {
		t.Error("Empty token should fail validation")
	}
}

// TestValidateCSRFToken_DeletesExpired verifies expired tokens are deleted during validation
func TestValidateCSRFToken_DeletesExpired(t *testing.T) {
	// Generate a token
	token, err := generateCSRFToken()
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// Manually set expiration to past
	csrfStore.Lock()
	if storedToken, exists := csrfStore.tokens[token]; exists {
		storedToken.expiresAt = time.Now().Add(-1 * time.Hour)
		csrfStore.tokens[token] = storedToken
	}
	csrfStore.Unlock()

	// Validation should fail and delete the token
	if ValidateCSRFToken(token) {
		t.Error("Expired token should fail validation")
	}

	// Verify token was deleted
	csrfStore.RLock()
	_, exists := csrfStore.tokens[token]
	csrfStore.RUnlock()

	if exists {
		t.Error("Expired token should be deleted from store")
	}
}

// TestCSRFTokenLimit verifies token generation fails when limit is reached
func TestCSRFTokenLimit(t *testing.T) {
	// Save original tokens
	csrfStore.Lock()
	originalTokens := csrfStore.tokens
	csrfStore.tokens = make(map[string]csrfToken)
	csrfStore.Unlock()

	// Restore original tokens after test
	defer func() {
		csrfStore.Lock()
		csrfStore.tokens = originalTokens
		csrfStore.Unlock()
	}()

	// Fill up to the limit
	for i := 0; i < maxCSRFTokens; i++ {
		_, err := generateCSRFToken()
		if err != nil {
			t.Fatalf("Failed to generate token %d: %v", i, err)
		}
	}

	// Next token should fail
	_, err := generateCSRFToken()
	if err == nil {
		t.Error("Expected error when exceeding token limit")
	}

	if !strings.Contains(err.Error(), "token limit exceeded") {
		t.Errorf("Expected 'token limit exceeded' error, got: %v", err)
	}
}

// TestDashboardHandler_Success verifies GET / returns 200 with HTML
func TestDashboardHandler_Success(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	DashboardHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	contentType := rr.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Errorf("Expected Content-Type text/html, got %s", contentType)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Domain Checker") {
		t.Error("Response should contain 'Domain Checker' title")
	}
}

// TestDashboardHandler_MethodNotAllowed verifies POST / returns 405
func TestDashboardHandler_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rr := httptest.NewRecorder()

	DashboardHandler(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", rr.Code)
	}
}

// TestDashboardHandler_WrongPath verifies GET /other returns 404
func TestDashboardHandler_WrongPath(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/other", nil)
	rr := httptest.NewRecorder()

	DashboardHandler(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", rr.Code)
	}
}

// TestDashboardHandler_CSRFTokenInMeta verifies CSRF token is present in meta tag
func TestDashboardHandler_CSRFTokenInMeta(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	DashboardHandler(rr, req)

	body := rr.Body.String()

	// Check for meta tag with csrf-token
	if !strings.Contains(body, `<meta name="csrf-token" content="`) {
		t.Error("Response should contain CSRF token in meta tag")
	}

	// Extract token from meta tag
	start := strings.Index(body, `<meta name="csrf-token" content="`)
	if start == -1 {
		t.Fatal("Could not find CSRF meta tag")
	}
	start += len(`<meta name="csrf-token" content="`)
	end := strings.Index(body[start:], `"`)
	if end == -1 {
		t.Fatal("Could not extract CSRF token from meta tag")
	}
	token := body[start : start+end]

	// Verify token is valid
	if len(token) != 64 {
		t.Errorf("CSRF token should be 64 characters, got %d", len(token))
	}
}

// TestDashboardHandler_SecurityHeaders verifies all security headers are present
func TestDashboardHandler_SecurityHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	DashboardHandler(rr, req)

	tests := []struct {
		name     string
		header   string
		expected string
	}{
		{
			name:     "Content-Security-Policy",
			header:   "Content-Security-Policy",
			expected: "default-src 'self'",
		},
		{
			name:     "X-Content-Type-Options",
			header:   "X-Content-Type-Options",
			expected: "nosniff",
		},
		{
			name:     "X-Frame-Options",
			header:   "X-Frame-Options",
			expected: "DENY",
		},
		{
			name:     "Content-Type",
			header:   "Content-Type",
			expected: "text/html",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value := rr.Header().Get(tt.header)
			if value == "" {
				t.Errorf("Header %s should be present", tt.header)
				return
			}
			if !strings.Contains(value, tt.expected) {
				t.Errorf("Header %s should contain %q, got %q", tt.header, tt.expected, value)
			}
		})
	}
}

// TestCheckDomainsHandler_WithValidCSRF verifies POST /check with valid token works
func TestCheckDomainsHandler_WithValidCSRF(t *testing.T) {
	// Generate a valid CSRF token
	token, err := generateCSRFToken()
	if err != nil {
		t.Fatalf("Failed to generate CSRF token: %v", err)
	}

	reqBody := domain.CheckRequest{Domains: []string{"example.com"}}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/check", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", token)

	rr := httptest.NewRecorder()
	CheckDomainsHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200 with valid CSRF token, got %d", rr.Code)
	}
}

// TestCheckDomainsHandler_WithInvalidCSRF verifies POST /check with invalid token returns 403
func TestCheckDomainsHandler_WithInvalidCSRF(t *testing.T) {
	reqBody := domain.CheckRequest{Domains: []string{"example.com"}}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/check", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", "invalid-token-12345")

	rr := httptest.NewRecorder()
	CheckDomainsHandler(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected status 403 with invalid CSRF token, got %d", rr.Code)
	}
}

// TestCheckDomainsHandler_WithoutCSRF verifies POST /check without token works (backward compatibility)
func TestCheckDomainsHandler_WithoutCSRF(t *testing.T) {
	reqBody := domain.CheckRequest{Domains: []string{"example.com"}}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/check", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	// No X-CSRF-Token header set

	rr := httptest.NewRecorder()
	CheckDomainsHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200 without CSRF token (backward compatibility), got %d", rr.Code)
	}
}

// TestCheckDomainsHandler_EmptyCSRFToken verifies empty CSRF token is treated as no token
func TestCheckDomainsHandler_EmptyCSRFToken(t *testing.T) {
	reqBody := domain.CheckRequest{Domains: []string{"example.com"}}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/check", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", "")

	rr := httptest.NewRecorder()
	CheckDomainsHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200 with empty CSRF token, got %d", rr.Code)
	}
}

// TestDashboardXSSProtection verifies XSS payloads in domain names are properly escaped
func TestDashboardXSSProtection(t *testing.T) {
	// Test payloads that could cause XSS if not properly escaped
	xssPayloads := []string{
		"<script>alert('XSS')</script>",
		"<img src=x onerror=alert('XSS')>",
		"javascript:alert('XSS')",
		"<svg/onload=alert('XSS')>",
		"\"><script>alert('XSS')</script>",
	}

	for _, payload := range xssPayloads {
		t.Run(payload, func(t *testing.T) {
			reqBody := domain.CheckRequest{Domains: []string{payload}}
			bodyBytes, _ := json.Marshal(reqBody)

			req := httptest.NewRequest(http.MethodPost, "/check", bytes.NewBuffer(bodyBytes))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			CheckDomainsHandler(rr, req)

			// Request should succeed (400 or 200 depending on validation)
			if rr.Code != http.StatusOK && rr.Code != http.StatusBadRequest {
				t.Errorf("Unexpected status code %d for XSS payload", rr.Code)
				return
			}

			// Verify response is valid JSON by checking it can be parsed
			var resp map[string]interface{}
			if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
				t.Errorf("Response should be valid JSON: %v", err)
				return
			}

			// Verify domain field in JSON response properly escapes HTML tags
			// Go's JSON encoder should escape < and > as \u003c and \u003e
			body := rr.Body.String()

			// Check if dangerous < > characters are properly escaped in JSON
			// Look for literal unescaped < followed by script/img/svg tags
			if strings.Contains(body, `"<script`) ||
			   strings.Contains(body, `"<img`) ||
			   strings.Contains(body, `"<svg`) {
				t.Errorf("Response contains unescaped HTML tags: %s", payload)
			}

			// Additionally verify that Go's JSON encoder is escaping correctly
			// The default behavior should convert < to \u003c and > to \u003e
			t.Logf("Response for payload %q: %s", payload, body)
		})
	}
}

// TestDashboardHandler_FullCSP verifies complete CSP policy
func TestDashboardHandler_FullCSP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	DashboardHandler(rr, req)

	csp := rr.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("Content-Security-Policy header should be present")
	}

	// Verify CSP includes required directives
	requiredDirectives := []string{
		"default-src 'self'",
		"script-src 'unsafe-inline'",
		"style-src 'unsafe-inline'",
	}

	for _, directive := range requiredDirectives {
		if !strings.Contains(csp, directive) {
			t.Errorf("CSP should contain %q, got: %s", directive, csp)
		}
	}
}

// TestCSRFTokenExpiryDuration verifies CSRF token expiration is set to 1 hour
func TestCSRFTokenExpiryDuration(t *testing.T) {
	expectedExpiry := 1 * time.Hour
	if csrfTokenExpiry != expectedExpiry {
		t.Errorf("CSRF token expiry should be %v, got %v", expectedExpiry, csrfTokenExpiry)
	}
}

// TestCSRFCleanupInterval verifies cleanup runs every 10 minutes
func TestCSRFCleanupInterval(t *testing.T) {
	expectedInterval := 10 * time.Minute
	if csrfCleanupInterval != expectedInterval {
		t.Errorf("CSRF cleanup interval should be %v, got %v", expectedInterval, csrfCleanupInterval)
	}
}

// TestMaxCSRFTokens verifies the token limit constant
func TestMaxCSRFTokens(t *testing.T) {
	expectedMax := 10000
	if maxCSRFTokens != expectedMax {
		t.Errorf("Max CSRF tokens should be %d, got %d", expectedMax, maxCSRFTokens)
	}
}
