package domain

import (
	"testing"
)

func TestNormalize(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      Domain
		wantErr   error
		wantErrIs bool // true if we should use errors.Is instead of equality
	}{
		// Basic functionality tests
		{
			name:  "bare name gets .com appended",
			input: "trucore",
			want: Domain{
				Full: "trucore.com",
				Name: "trucore",
				TLD:  "com",
			},
			wantErr: nil,
		},
		{
			name:  "domain with .io preserved",
			input: "foo.io",
			want: Domain{
				Full: "foo.io",
				Name: "foo",
				TLD:  "io",
			},
			wantErr: nil,
		},
		{
			name:  "domain with .org preserved (BUG FIX TEST)",
			input: "example.org",
			want: Domain{
				Full: "example.org",
				Name: "example",
				TLD:  "org",
			},
			wantErr: nil,
		},
		{
			name:  "domain with .net preserved",
			input: "test.net",
			want: Domain{
				Full: "test.net",
				Name: "test",
				TLD:  "net",
			},
			wantErr: nil,
		},
		{
			name:  ".com domain preserved as-is",
			input: "example.com",
			want: Domain{
				Full: "example.com",
				Name: "example",
				TLD:  "com",
			},
			wantErr: nil,
		},

		// Case normalization tests
		{
			name:  "uppercase converted to lowercase",
			input: "TRUCORE",
			want: Domain{
				Full: "trucore.com",
				Name: "trucore",
				TLD:  "com",
			},
			wantErr: nil,
		},
		{
			name:  "mixed case converted to lowercase",
			input: "TruCore.COM",
			want: Domain{
				Full: "trucore.com",
				Name: "trucore",
				TLD:  "com",
			},
			wantErr: nil,
		},

		// Whitespace handling tests
		{
			name:  "leading whitespace trimmed",
			input: "  trucore",
			want: Domain{
				Full: "trucore.com",
				Name: "trucore",
				TLD:  "com",
			},
			wantErr: nil,
		},
		{
			name:  "trailing whitespace trimmed",
			input: "trucore  ",
			want: Domain{
				Full: "trucore.com",
				Name: "trucore",
				TLD:  "com",
			},
			wantErr: nil,
		},
		{
			name:  "leading and trailing whitespace trimmed",
			input: "  trucore  ",
			want: Domain{
				Full: "trucore.com",
				Name: "trucore",
				TLD:  "com",
			},
			wantErr: nil,
		},
		{
			name:  "whitespace around domain with TLD",
			input: "  example.org  ",
			want: Domain{
				Full: "example.org",
				Name: "example",
				TLD:  "org",
			},
			wantErr: nil,
		},

		// Subdomain tests
		{
			name:  "subdomain with .com",
			input: "sub.example.com",
			want: Domain{
				Full: "sub.example.com",
				Name: "sub.example",
				TLD:  "com",
			},
			wantErr: nil,
		},
		{
			name:  "subdomain with .org",
			input: "api.service.org",
			want: Domain{
				Full: "api.service.org",
				Name: "api.service",
				TLD:  "org",
			},
			wantErr: nil,
		},
		{
			name:  "deep subdomain",
			input: "a.b.c.d.example.com",
			want: Domain{
				Full: "a.b.c.d.example.com",
				Name: "a.b.c.d.example",
				TLD:  "com",
			},
			wantErr: nil,
		},

		// Error cases
		{
			name:    "empty string returns error",
			input:   "",
			want:    Domain{},
			wantErr: ErrEmptyDomain,
		},
		{
			name:    "whitespace only returns error",
			input:   "   ",
			want:    Domain{},
			wantErr: ErrEmptyDomain,
		},
		{
			name:    "trailing dot returns error",
			input:   "example.",
			want:    Domain{},
			wantErr: ErrInvalidFormat,
		},
		{
			name:    "trailing dot with TLD returns error",
			input:   "example.com.",
			want:    Domain{},
			wantErr: ErrInvalidFormat,
		},
		{
			name:    "single dot returns error",
			input:   ".",
			want:    Domain{},
			wantErr: ErrInvalidFormat,
		},
		{
			name:    "leading dot returns error (after .com append)",
			input:   ".example",
			want:    Domain{},
			wantErr: ErrInvalidFormat,
		},

		// Special characters (allowed in domains)
		{
			name:  "hyphen in domain name",
			input: "my-domain",
			want: Domain{
				Full: "my-domain.com",
				Name: "my-domain",
				TLD:  "com",
			},
			wantErr: nil,
		},
		{
			name:  "hyphen in domain with TLD",
			input: "my-domain.io",
			want: Domain{
				Full: "my-domain.io",
				Name: "my-domain",
				TLD:  "io",
			},
			wantErr: nil,
		},
		{
			name:  "numbers in domain",
			input: "domain123",
			want: Domain{
				Full: "domain123.com",
				Name: "domain123",
				TLD:  "com",
			},
			wantErr: nil,
		},

		// Country code TLDs
		{
			name:  "country code TLD preserved (.uk)",
			input: "example.co.uk",
			want: Domain{
				Full: "example.co.uk",
				Name: "example.co",
				TLD:  "uk",
			},
			wantErr: nil,
		},
		{
			name:  "country code TLD preserved (.au)",
			input: "example.com.au",
			want: Domain{
				Full: "example.com.au",
				Name: "example.com",
				TLD:  "au",
			},
			wantErr: nil,
		},

		// New gTLDs
		{
			name:  "new gTLD .dev preserved",
			input: "myapp.dev",
			want: Domain{
				Full: "myapp.dev",
				Name: "myapp",
				TLD:  "dev",
			},
			wantErr: nil,
		},
		{
			name:  "new gTLD .app preserved",
			input: "myapp.app",
			want: Domain{
				Full: "myapp.app",
				Name: "myapp",
				TLD:  "app",
			},
			wantErr: nil,
		},
		{
			name:  "new gTLD .ai preserved",
			input: "startup.ai",
			want: Domain{
				Full: "startup.ai",
				Name: "startup",
				TLD:  "ai",
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Normalize(tt.input)

			// Check error
			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("Normalize(%q) error = nil, want %v", tt.input, tt.wantErr)
					return
				}
				if err != tt.wantErr {
					t.Errorf("Normalize(%q) error = %v, want %v", tt.input, err, tt.wantErr)
				}
				return
			}

			// Check no error when not expected
			if err != nil {
				t.Errorf("Normalize(%q) unexpected error: %v", tt.input, err)
				return
			}

			// Check result
			if got != tt.want {
				t.Errorf("Normalize(%q) = %+v, want %+v", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeBatch(t *testing.T) {
	tests := []struct {
		name        string
		inputs      []string
		wantDomains []Domain
		wantErrs    []error
	}{
		{
			name:   "multiple valid domains",
			inputs: []string{"trucore", "example.org", "foo.io"},
			wantDomains: []Domain{
				{Full: "trucore.com", Name: "trucore", TLD: "com"},
				{Full: "example.org", Name: "example", TLD: "org"},
				{Full: "foo.io", Name: "foo", TLD: "io"},
			},
			wantErrs: []error{nil, nil, nil},
		},
		{
			name:   "mix of valid and invalid domains",
			inputs: []string{"valid", "", "example.com"},
			wantDomains: []Domain{
				{Full: "valid.com", Name: "valid", TLD: "com"},
				{}, // empty domain
				{Full: "example.com", Name: "example", TLD: "com"},
			},
			wantErrs: []error{nil, ErrEmptyDomain, nil},
		},
		{
			name:        "empty input slice",
			inputs:      []string{},
			wantDomains: []Domain{},
			wantErrs:    []error{},
		},
		{
			name:   "all invalid domains",
			inputs: []string{"", "   ", "example."},
			wantDomains: []Domain{
				{}, // empty
				{}, // whitespace
				{}, // trailing dot
			},
			wantErrs: []error{ErrEmptyDomain, ErrEmptyDomain, ErrInvalidFormat},
		},
		{
			name:   "case normalization in batch",
			inputs: []string{"UPPER", "MiXeD.ORG", "  spaced  "},
			wantDomains: []Domain{
				{Full: "upper.com", Name: "upper", TLD: "com"},
				{Full: "mixed.org", Name: "mixed", TLD: "org"},
				{Full: "spaced.com", Name: "spaced", TLD: "com"},
			},
			wantErrs: []error{nil, nil, nil},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotDomains, gotErrs := NormalizeBatch(tt.inputs)

			// Check slice lengths
			if len(gotDomains) != len(tt.wantDomains) {
				t.Errorf("NormalizeBatch(%v) returned %d domains, want %d",
					tt.inputs, len(gotDomains), len(tt.wantDomains))
				return
			}
			if len(gotErrs) != len(tt.wantErrs) {
				t.Errorf("NormalizeBatch(%v) returned %d errors, want %d",
					tt.inputs, len(gotErrs), len(tt.wantErrs))
				return
			}

			// Check each domain and error
			for i := range tt.inputs {
				if gotDomains[i] != tt.wantDomains[i] {
					t.Errorf("NormalizeBatch(%v) domain[%d] = %+v, want %+v",
						tt.inputs, i, gotDomains[i], tt.wantDomains[i])
				}

				if gotErrs[i] != tt.wantErrs[i] {
					t.Errorf("NormalizeBatch(%v) error[%d] = %v, want %v",
						tt.inputs, i, gotErrs[i], tt.wantErrs[i])
				}
			}
		})
	}
}

// TestNormalizeEdgeCases tests additional edge cases
func TestNormalizeEdgeCases(t *testing.T) {
	// Test: Bug fix verification - non-.com TLDs should NOT get .com appended
	bugFixTests := []struct {
		input string
		want  string
	}{
		{"example.org", "example.org"},     // should NOT become example.org.com
		{"test.net", "test.net"},           // should NOT become test.net.com
		{"mysite.io", "mysite.io"},         // should NOT become mysite.io.com
		{"app.dev", "app.dev"},             // should NOT become app.dev.com
		{"site.ai", "site.ai"},             // should NOT become site.ai.com
		{"example.co.uk", "example.co.uk"}, // should NOT become example.co.uk.com
	}

	for _, tt := range bugFixTests {
		t.Run("BugFix_"+tt.input, func(t *testing.T) {
			got, err := Normalize(tt.input)
			if err != nil {
				t.Fatalf("Normalize(%q) unexpected error: %v", tt.input, err)
			}
			if got.Full != tt.want {
				t.Errorf("BUG FIX FAILED: Normalize(%q).Full = %q, want %q (TLD should be preserved, not have .com appended)",
					tt.input, got.Full, tt.want)
			}
		})
	}
}
