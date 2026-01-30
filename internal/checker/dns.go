// Package checker provides domain availability checking functionality.
package checker

import (
	"context"
	"net"
	"time"

	"domaincheck/internal/domain"
)

// DNSFilter performs a DNS pre-filter check to determine if a domain has DNS records.
// This is used to quickly identify registered domains before expensive RDAP/WHOIS lookups.
//
// Logic:
//   - If domain HAS DNS records (A/AAAA/MX/NS) → likely registered → shouldSkip=true
//   - If domain has NO DNS records → might be available or parked → shouldSkip=false (need RDAP/WHOIS)
//   - If DNS lookup errors → be cautious → shouldSkip=false (need RDAP/WHOIS)
//
// Returns:
//   - likelyAvailable: true if no DNS records found (preliminary indicator only)
//   - shouldSkip: true if we can skip RDAP/WHOIS (domain definitely has records)
//   - err: any unexpected errors (nil for normal operation)
func DNSFilter(ctx context.Context, d domain.Domain) (likelyAvailable bool, shouldSkip bool, err error) {
	// Set timeout for DNS lookups
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	resolver := &net.Resolver{}

	// Check A/AAAA records (IP addresses)
	ips, err := resolver.LookupIP(ctx, "ip", d.Full)
	if err == nil && len(ips) > 0 {
		// Domain has IP records → definitely registered
		return false, true, nil
	}

	// Check MX records (mail servers)
	mxRecords, err := resolver.LookupMX(ctx, d.Full)
	if err == nil && len(mxRecords) > 0 {
		// Domain has MX records → definitely registered
		return false, true, nil
	}

	// Check NS records (nameservers)
	nsRecords, err := resolver.LookupNS(ctx, d.Full)
	if err == nil && len(nsRecords) > 0 {
		// Domain has NS records → definitely registered
		return false, true, nil
	}

	// No DNS records found → might be available, but need RDAP/WHOIS to be sure
	// (could be registered but not configured)
	return true, false, nil
}
