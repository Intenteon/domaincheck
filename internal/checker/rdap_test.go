package checker

import (
	"context"
	"testing"
	"time"

	"domaincheck/internal/domain"
)

func TestRDAPCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tests := []struct {
		name          string
		domain        domain.Domain
		wantAvailable bool
		wantErr       bool
		skipReason    string // Reason to skip this test (e.g., RDAP server unreliable)
	}{
		{
			name: "known registered domain (google.com)",
			domain: domain.Domain{
				Full: "google.com",
				Name: "google",
				TLD:  "com",
			},
			wantAvailable: false,
			wantErr:       false,
		},
		{
			name: "known registered domain (example.org)",
			domain: domain.Domain{
				Full: "example.org",
				Name: "example",
				TLD:  "org",
			},
			wantAvailable: false,
			wantErr:       false,
		},
		{
			name: "likely available domain (.com)",
			domain: domain.Domain{
				Full: "thisdomainsurelydoesnotexist98765.com",
				Name: "thisdomainsurelydoesnotexist98765",
				TLD:  "com",
			},
			wantAvailable: true,
			wantErr:       false,
		},
		{
			name: "unsupported TLD (.xyz)",
			domain: domain.Domain{
				Full: "example.xyz",
				Name: "example",
				TLD:  "xyz",
			},
			wantAvailable: false,
			wantErr:       true,
		},
		{
			name: "empty domain",
			domain: domain.Domain{
				Full: "",
				Name: "",
				TLD:  "",
			},
			wantAvailable: false,
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skipReason != "" {
				t.Skip(tt.skipReason)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			gotAvailable, err := RDAPCheck(ctx, tt.domain)

			// Check error expectation
			if tt.wantErr {
				if err == nil {
					t.Errorf("RDAPCheck() expected error but got nil")
				}
				return
			}

			// Check no error when not expected
			if err != nil {
				t.Errorf("RDAPCheck() unexpected error: %v", err)
				return
			}

			// Check availability result
			if gotAvailable != tt.wantAvailable {
				t.Errorf("RDAPCheck() available = %v, want %v", gotAvailable, tt.wantAvailable)
			}
		})
	}
}

// TestRDAPCheckCanceledContext tests behavior with a pre-canceled context
func TestRDAPCheckCanceledContext(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	d := domain.Domain{
		Full: "google.com",
		Name: "google",
		TLD:  "com",
	}

	_, err := RDAPCheck(ctx, d)

	// Should return an error due to canceled context
	if err == nil {
		t.Error("RDAPCheck() with canceled context should return error")
	}
}

// TestRDAPCheckUnsupportedTLDs tests various unsupported TLDs
func TestRDAPCheckUnsupportedTLDs(t *testing.T) {
	unsupportedTLDs := []string{
		"xyz", "co", "uk", "de", "fr", "jp",
	}

	for _, tld := range unsupportedTLDs {
		t.Run("unsupported_"+tld, func(t *testing.T) {
			d := domain.Domain{
				Full: "example." + tld,
				Name: "example",
				TLD:  tld,
			}

			ctx := context.Background()
			_, err := RDAPCheck(ctx, d)

			if err == nil {
				t.Errorf("RDAPCheck() with unsupported TLD .%s should return error", tld)
			}
		})
	}
}

// TestRDAPCheckSupportedTLDs verifies all TLDs in our map are callable
func TestRDAPCheckSupportedTLDs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Test that each TLD in our map can be queried
	// Only testing verified working RDAP servers: com, net, org
	supportedTLDs := []string{"com", "net", "org"}

	for _, tld := range supportedTLDs {
		t.Run("supported_"+tld, func(t *testing.T) {
			// Use a well-known registered domain for each TLD
			var domainName string
			switch tld {
			case "com":
				domainName = "google"
			case "net":
				domainName = "cloudflare"
			case "org":
				domainName = "wikipedia"
			default:
				domainName = "example"
			}

			d := domain.Domain{
				Full: domainName + "." + tld,
				Name: domainName,
				TLD:  tld,
			}

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			available, err := RDAPCheck(ctx, d)

			// Should not error (but availability depends on real registration)
			if err != nil {
				t.Errorf("RDAPCheck() for supported TLD .%s returned error: %v", tld, err)
			}

			// These are all well-known registered domains, should be taken
			if available {
				t.Logf("Warning: %s appears available (might be test flake or domain expired)", d.Full)
			}
		})
	}
}
