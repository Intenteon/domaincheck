package checker

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"domaincheck/internal/domain"
)

// rdapServers maps TLDs to their RDAP server base URLs.
// These are verified working RDAP servers for each TLD.
// Note: Many TLDs use the RDAP Bootstrap service, but we hardcode known servers for performance.
var rdapServers = map[string]string{
	"com": "https://rdap.verisign.com/com/v1/domain/",
	"net": "https://rdap.verisign.com/net/v1/domain/",
	"org": "https://rdap.publicinterestregistry.org/rdap/domain/",
	// Note: .io, .ai, .dev, .app servers may require bootstrap service
	// Removed for now until verified - can be added when correct URLs are confirmed
}

// rdapResponse represents the minimal RDAP JSON response we care about.
// Full RDAP responses contain much more data, but we only need the status.
type rdapResponse struct {
	// Status contains registration status values like "active", "registered", etc.
	Status []string `json:"status"`
}

// RDAPCheck queries an RDAP server to check if a domain is registered.
//
// RDAP (Registration Data Access Protocol) is the modern replacement for WHOIS.
// It provides structured JSON responses and is the preferred method for checking domains.
//
// The function:
//   - Determines the correct RDAP server based on the domain's TLD
//   - Makes an HTTPS request to the RDAP server
//   - Parses the JSON response
//   - Returns availability based on HTTP status code and response content
//
// HTTP Status Code Interpretation:
//   - 404 Not Found → Domain is available
//   - 200 OK → Domain exists, check status array
//   - Other codes → Error occurred
//
// Returns:
//   - available: true if domain is available for registration
//   - err: error if RDAP query failed or TLD is not supported
func RDAPCheck(ctx context.Context, d domain.Domain) (available bool, err error) {
	// Find RDAP server for this TLD
	serverBase, ok := rdapServers[d.TLD]
	if !ok {
		return false, fmt.Errorf("RDAP server not configured for TLD: %s", d.TLD)
	}

	// Construct full RDAP URL
	url := serverBase + d.Full

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create RDAP request: %w", err)
	}

	// Set User-Agent header (some RDAP servers require this)
	req.Header.Set("User-Agent", "domaincheck/1.0")
	req.Header.Set("Accept", "application/rdap+json")

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("RDAP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Interpret HTTP status code
	switch resp.StatusCode {
	case http.StatusNotFound:
		// 404 = domain not found in registry = available
		return true, nil

	case http.StatusOK:
		// 200 = domain found, parse response to check status
		var rdapResp rdapResponse
		if err := json.NewDecoder(resp.Body).Decode(&rdapResp); err != nil {
			return false, fmt.Errorf("failed to parse RDAP response: %w", err)
		}

		// Check status array for registration indicators
		// Common statuses: "active", "registered", "client*", "server*"
		// If status array is empty or contains only inactive statuses, might be available
		if len(rdapResp.Status) == 0 {
			// No status means likely available (rare but possible)
			return true, nil
		}

		// Check for active/registered status
		for _, status := range rdapResp.Status {
			// These statuses indicate the domain is registered
			if status == "active" || status == "registered" {
				return false, nil
			}
		}

		// If no clear "registered" status, assume taken to be safe
		return false, nil

	default:
		// Other status codes indicate errors
		return false, fmt.Errorf("RDAP server returned unexpected status: %d", resp.StatusCode)
	}
}
