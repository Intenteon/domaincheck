package checker

import (
	"context"
	"testing"

	"domaincheck/internal/domain"
)

func TestDNSFilter(t *testing.T) {
	tests := []struct {
		name             string
		domain           domain.Domain
		wantAvailable    bool
		wantSkip         bool
		expectErr        bool
		setupContext     func() context.Context
	}{
		{
			name: "domain with A records (google.com)",
			domain: domain.Domain{
				Full: "google.com",
				Name: "google",
				TLD:  "com",
			},
			wantAvailable: false,
			wantSkip:      true,
			expectErr:     false,
			setupContext:  func() context.Context { return context.Background() },
		},
		{
			name: "domain with MX records (gmail.com)",
			domain: domain.Domain{
				Full: "gmail.com",
				Name: "gmail",
				TLD:  "com",
			},
			wantAvailable: false,
			wantSkip:      true,
			expectErr:     false,
			setupContext:  func() context.Context { return context.Background() },
		},
		{
			name: "domain with NS records (cloudflare.com)",
			domain: domain.Domain{
				Full: "cloudflare.com",
				Name: "cloudflare",
				TLD:  "com",
			},
			wantAvailable: false,
			wantSkip:      true,
			expectErr:     false,
			setupContext:  func() context.Context { return context.Background() },
		},
		{
			name: "non-existent domain",
			domain: domain.Domain{
				Full: "thisdomaindefinitelydoesnotexist12345.com",
				Name: "thisdomaindefinitelydoesnotexist12345",
				TLD:  "com",
			},
			wantAvailable: true,
			wantSkip:      false,
			expectErr:     false,
			setupContext:  func() context.Context { return context.Background() },
		},
		{
			name: "invalid domain format",
			domain: domain.Domain{
				Full: "invalid..domain",
				Name: "invalid.",
				TLD:  "domain",
			},
			wantAvailable: true,
			wantSkip:      false,
			expectErr:     false,
			setupContext:  func() context.Context { return context.Background() },
		},
		{
			name: "subdomain with records",
			domain: domain.Domain{
				Full: "www.google.com",
				Name: "www.google",
				TLD:  "com",
			},
			wantAvailable: false,
			wantSkip:      true,
			expectErr:     false,
			setupContext:  func() context.Context { return context.Background() },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setupContext()
			gotAvailable, gotSkip, gotErr := DNSFilter(ctx, tt.domain)

			// Check error expectation
			if tt.expectErr && gotErr == nil {
				t.Errorf("DNSFilter() expected error but got nil")
			}
			if !tt.expectErr && gotErr != nil {
				t.Errorf("DNSFilter() unexpected error: %v", gotErr)
			}

			// Check results (only if no error expected)
			if !tt.expectErr {
				if gotAvailable != tt.wantAvailable {
					t.Errorf("DNSFilter() likelyAvailable = %v, want %v", gotAvailable, tt.wantAvailable)
				}
				if gotSkip != tt.wantSkip {
					t.Errorf("DNSFilter() shouldSkip = %v, want %v", gotSkip, tt.wantSkip)
				}
			}
		})
	}
}

// TestDNSFilterCanceledContext tests behavior with a pre-canceled context
func TestDNSFilterCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	d := domain.Domain{
		Full: "google.com",
		Name: "google",
		TLD:  "com",
	}

	// Should handle canceled context gracefully
	// Since DNS operations will fail, should return likelyAvailable=true, shouldSkip=false
	gotAvailable, gotSkip, gotErr := DNSFilter(ctx, d)

	if gotErr != nil {
		t.Errorf("DNSFilter() with canceled context should not return error, got: %v", gotErr)
	}

	// When DNS fails (including due to canceled context), we can't determine availability
	// so we return likelyAvailable=true, shouldSkip=false to trigger RDAP/WHOIS
	if !gotAvailable {
		t.Errorf("DNSFilter() with canceled context likelyAvailable = %v, want true", gotAvailable)
	}
	if gotSkip {
		t.Errorf("DNSFilter() with canceled context shouldSkip = %v, want false", gotSkip)
	}
}

// TestDNSFilterEmptyDomain tests behavior with empty domain
func TestDNSFilterEmptyDomain(t *testing.T) {
	ctx := context.Background()
	d := domain.Domain{
		Full: "",
		Name: "",
		TLD:  "",
	}

	// Empty domain should be handled gracefully
	gotAvailable, gotSkip, gotErr := DNSFilter(ctx, d)

	if gotErr != nil {
		t.Errorf("DNSFilter() with empty domain should not return error, got: %v", gotErr)
	}

	// Empty domain won't resolve, so likelyAvailable=true, shouldSkip=false
	if !gotAvailable {
		t.Errorf("DNSFilter() with empty domain likelyAvailable = %v, want true", gotAvailable)
	}
	if gotSkip {
		t.Errorf("DNSFilter() with empty domain shouldSkip = %v, want false", gotSkip)
	}
}
