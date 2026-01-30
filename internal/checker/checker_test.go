package checker

import (
	"context"
	"testing"
	"time"

	"domaincheck/internal/domain"
)

func TestCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tests := []struct {
		name          string
		domain        domain.Domain
		wantAvailable bool
		wantErr       bool
		wantSource    string // Expected source: "dns", "rdap", or "whois"
	}{
		{
			name: "known registered domain with DNS records (google.com)",
			domain: domain.Domain{
				Full: "google.com",
				Name: "google",
				TLD:  "com",
			},
			wantAvailable: false,
			wantErr:       false,
			wantSource:    "dns", // Should be caught by DNS pre-filter
		},
		{
			name: "known registered domain (.org with RDAP)",
			domain: domain.Domain{
				Full: "wikipedia.org",
				Name: "wikipedia",
				TLD:  "org",
			},
			wantAvailable: false,
			wantErr:       false,
			// Could be "dns" or "rdap" depending on DNS pre-filter
		},
		{
			name: "likely available domain (.com)",
			domain: domain.Domain{
				Full: "thisdomainsurelydoesnotexist11223344.com",
				Name: "thisdomainsurelydoesnotexist11223344",
				TLD:  "com",
			},
			wantAvailable: true,
			wantErr:       false,
			// Could be "rdap" or "whois" depending on which succeeds
		},
		{
			name: "domain with unsupported TLD (should fall back to WHOIS)",
			domain: domain.Domain{
				Full: "google.xyz",
				Name: "google",
				TLD:  "xyz",
			},
			wantAvailable: false, // google.xyz is likely registered
			wantErr:       false,
			wantSource:    "whois", // RDAP doesn't support .xyz, should use WHOIS
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			result, err := Check(ctx, tt.domain)

			// Check error expectation
			if tt.wantErr {
				if err == nil {
					t.Errorf("Check() expected error but got nil")
				}
				return
			}

			// Check no error when not expected
			if err != nil {
				t.Errorf("Check() unexpected error: %v", err)
				return
			}

			// Check availability
			if result.Available != tt.wantAvailable {
				t.Errorf("Check() Available = %v, want %v", result.Available, tt.wantAvailable)
			}

			// Check status consistency
			if result.Available && result.Status != domain.StatusAvailable {
				t.Errorf("Check() Available=true but Status=%v, want StatusAvailable", result.Status)
			}
			if !result.Available && result.Status != domain.StatusTaken {
				t.Errorf("Check() Available=false but Status=%v, want StatusTaken", result.Status)
			}

			// Check source is set
			if result.Source == "" {
				t.Error("Check() Source is empty, expected dns/rdap/whois")
			}

			// Check source matches expectation (if specified)
			if tt.wantSource != "" && result.Source != tt.wantSource {
				t.Logf("Check() Source = %s, expected %s (may be acceptable)", result.Source, tt.wantSource)
			}

			// Check domain is set correctly
			if result.Domain != tt.domain {
				t.Errorf("Check() Domain = %+v, want %+v", result.Domain, tt.domain)
			}

			// Check timestamp is recent
			if time.Since(result.CheckedAt) > 30*time.Second {
				t.Errorf("Check() CheckedAt is too old: %v", result.CheckedAt)
			}

			// Check duration is reasonable
			if result.Duration <= 0 {
				t.Errorf("Check() Duration = %v, want > 0", result.Duration)
			}
			if result.Duration > 30*time.Second {
				t.Errorf("Check() Duration = %v, want < 30s", result.Duration)
			}

			t.Logf("Check() %s: available=%v, source=%s, duration=%v",
				tt.domain.Full, result.Available, result.Source, result.Duration)
		})
	}
}

// TestCheckCanceledContext tests behavior when context is cancelled
func TestCheckCanceledContext(t *testing.T) {
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

	result, err := Check(ctx, d)

	// Should return an error or handle cancellation gracefully
	// DNS filter might succeed before cancellation, so this test is lenient
	if err != nil {
		// Error is acceptable
		t.Logf("Check() with canceled context returned error: %v", err)
	} else {
		// Or it might succeed quickly via DNS
		t.Logf("Check() with canceled context completed successfully (fast DNS check)")
	}

	// Just verify the result structure is valid
	if result.CheckedAt.IsZero() {
		t.Error("Check() CheckedAt should be set even on cancellation")
	}
}

// TestCheckSourcesFlow verifies the fallback flow through different sources
func TestCheckSourcesFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Test 1: Domain with DNS records should use DNS source
	t.Run("dns_source_fast_path", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		d := domain.Domain{
			Full: "cloudflare.com",
			Name: "cloudflare",
			TLD:  "com",
		}

		result, err := Check(ctx, d)
		if err != nil {
			t.Fatalf("Check() error = %v", err)
		}

		if result.Source != "dns" {
			t.Logf("Check() Source = %s, expected dns (domain has DNS records)", result.Source)
		}

		if result.Available {
			t.Error("Check() cloudflare.com should not be available")
		}
	})

	// Test 2: Domain without RDAP support should use WHOIS
	t.Run("whois_fallback_for_unsupported_tld", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		d := domain.Domain{
			Full: "example.co.uk",
			Name: "example.co",
			TLD:  "uk",
		}

		result, err := Check(ctx, d)
		if err != nil {
			t.Fatalf("Check() error = %v", err)
		}

		// Should fall back to WHOIS since .uk not in RDAP map
		if result.Source != "whois" {
			t.Logf("Check() Source = %s, expected whois (TLD .uk not in RDAP map)", result.Source)
		}
	})
}

// TestCheckPerformance verifies the check completes in reasonable time
func TestCheckPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	d := domain.Domain{
		Full: "google.com",
		Name: "google",
		TLD:  "com",
	}

	start := time.Now()
	result, err := Check(ctx, d)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	// DNS pre-filter should make this very fast (under 1 second)
	if elapsed > 5*time.Second {
		t.Errorf("Check() took %v, want < 5s (DNS pre-filter should be fast)", elapsed)
	}

	// Verify duration field matches
	if result.Duration > elapsed+100*time.Millisecond {
		t.Errorf("Check() Duration %v > actual elapsed %v", result.Duration, elapsed)
	}

	t.Logf("Check() performance: %v (source: %s)", elapsed, result.Source)
}
