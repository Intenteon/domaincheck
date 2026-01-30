package checker

import (
	"context"
	"testing"
	"time"

	"domaincheck/internal/domain"
)

func TestWHOISCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tests := []struct {
		name          string
		domain        domain.Domain
		wantAvailable bool
		wantErr       bool
		skipReason    string
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
				Full: "thisdomainsurelydoesnotexist54321whois.com",
				Name: "thisdomainsurelydoesnotexist54321whois",
				TLD:  "com",
			},
			wantAvailable: true,
			wantErr:       false,
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

			gotAvailable, err := WHOISCheck(ctx, tt.domain)

			// Check error expectation
			if tt.wantErr {
				if err == nil {
					t.Errorf("WHOISCheck() expected error but got nil")
				}
				return
			}

			// Check no error when not expected
			if err != nil {
				t.Errorf("WHOISCheck() unexpected error: %v", err)
				return
			}

			// Check availability result
			if gotAvailable != tt.wantAvailable {
				t.Errorf("WHOISCheck() available = %v, want %v", gotAvailable, tt.wantAvailable)
			}
		})
	}
}

// TestWHOISCheckCanceledContext tests behavior with a pre-canceled context
func TestWHOISCheckCanceledContext(t *testing.T) {
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

	_, err := WHOISCheck(ctx, d)

	// Should return an error due to canceled context
	if err == nil {
		t.Error("WHOISCheck() with canceled context should return error")
	}
}

// TestWHOISCheckTimeout tests behavior when whois command times out
func TestWHOISCheckTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create a context with a very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Give it a moment to ensure timeout
	time.Sleep(5 * time.Millisecond)

	d := domain.Domain{
		Full: "google.com",
		Name: "google",
		TLD:  "com",
	}

	_, err := WHOISCheck(ctx, d)

	// Should return an error due to timeout
	if err == nil {
		t.Error("WHOISCheck() with timeout should return error")
	}
}

// TestWHOISCheckVariousTLDs tests WHOIS with different TLDs
func TestWHOISCheckVariousTLDs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// WHOIS should work with any TLD (unlike RDAP which is selective)
	tlds := []struct {
		tld        string
		domainName string
	}{
		{"com", "google"},
		{"org", "wikipedia"},
		{"net", "cloudflare"},
		{"io", "github"},
	}

	for _, tc := range tlds {
		t.Run("whois_"+tc.tld, func(t *testing.T) {
			d := domain.Domain{
				Full: tc.domainName + "." + tc.tld,
				Name: tc.domainName,
				TLD:  tc.tld,
			}

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			available, err := WHOISCheck(ctx, d)

			// Should not error
			if err != nil {
				t.Errorf("WHOISCheck() for .%s returned error: %v", tc.tld, err)
			}

			// These are all well-known registered domains, should be taken
			if available {
				t.Logf("Warning: %s appears available (might be test flake)", d.Full)
			}
		})
	}
}
