// Package checker provides domain availability checking functionality.
package checker

import (
	"context"
	"fmt"
	"time"

	"domaincheck/internal/domain"
)

// Check orchestrates the domain availability checking process.
//
// The checking flow is optimized for speed and reliability:
//
//  1. DNS Pre-Filter (fastest, ~100ms):
//     - Checks if domain has DNS records (A/AAAA/MX/NS)
//     - If records exist → domain is definitely registered → DONE
//     - If no records → might be available, need further checks
//
//  2. RDAP Check (fast, structured, ~500ms):
//     - Queries RDAP server for domain registration data
//     - Returns structured JSON response
//     - If supported TLD → use RDAP result → DONE
//     - If unsupported TLD → fall back to WHOIS
//
//  3. WHOIS Check (slow, unstructured, ~1-2s):
//     - Executes whois command and parses text output
//     - Works with any TLD but less reliable
//     - Last resort fallback
//
// The function returns a domain.Result containing:
//   - Status: StatusAvailable, StatusTaken, or StatusError
//   - Available: convenience boolean
//   - Source: which protocol provided the answer ("dns", "rdap", "whois")
//   - CheckedAt: timestamp when check started
//   - Duration: total time taken
//   - Error: error message if any
//
// Context Handling:
// The context is propagated to all sub-checks. If the context is cancelled or
// times out, the check will abort and return an error.
//
// Example Usage:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
//	defer cancel()
//
//	d := domain.Domain{Full: "example.com", Name: "example", TLD: "com"}
//	result, err := checker.Check(ctx, d)
//	if err != nil {
//	    log.Fatalf("Check failed: %v", err)
//	}
//	if result.Available {
//	    fmt.Println("Domain is available!")
//	}
func Check(ctx context.Context, d domain.Domain) (domain.Result, error) {
	start := time.Now()

	result := domain.Result{
		Domain:    d,
		CheckedAt: start,
	}

	// Step 1: DNS Pre-Filter
	// This is the fastest check - if domain has DNS records, it's definitely registered
	_, shouldSkip, err := DNSFilter(ctx, d)
	if err != nil {
		// DNS filter failed - continue with RDAP/WHOIS to be thorough
		// Don't return error, just log it internally and continue
	} else if shouldSkip {
		// Domain has DNS records - it's taken, no need for RDAP/WHOIS
		result.Status = domain.StatusTaken
		result.Available = false
		result.Source = "dns"
		result.Duration = time.Since(start)
		return result, nil
	}

	// Step 2: Try RDAP Check
	// DNS said "no records" (might be available), so check RDAP for definitive answer
	available, err := RDAPCheck(ctx, d)
	if err == nil {
		// RDAP succeeded
		if available {
			result.Status = domain.StatusAvailable
			result.Available = true
		} else {
			result.Status = domain.StatusTaken
			result.Available = false
		}
		result.Source = "rdap"
		result.Duration = time.Since(start)
		return result, nil
	}

	// RDAP failed - could be unsupported TLD or server error
	// Fall back to WHOIS as last resort

	// Step 3: WHOIS Fallback
	available, err = WHOISCheck(ctx, d)
	if err != nil {
		// WHOIS also failed - return error
		result.Status = domain.StatusError
		result.Available = false
		result.Error = fmt.Sprintf("all checks failed, last error: %v", err)
		result.Source = "whois"
		result.Duration = time.Since(start)
		return result, fmt.Errorf("domain check failed: %w", err)
	}

	// WHOIS succeeded
	if available {
		result.Status = domain.StatusAvailable
		result.Available = true
	} else {
		result.Status = domain.StatusTaken
		result.Available = false
	}
	result.Source = "whois"
	result.Duration = time.Since(start)
	return result, nil
}
