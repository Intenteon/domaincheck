// Package domain provides core domain-related types and operations.
package domain

import (
	"errors"
	"strings"
)

var (
	// ErrEmptyDomain is returned when the input domain is empty or whitespace-only
	ErrEmptyDomain = errors.New("domain cannot be empty")

	// ErrInvalidFormat is returned when the domain format is invalid
	ErrInvalidFormat = errors.New("invalid domain format")
)

// Normalize converts a user input string into a validated Domain.
//
// Normalization rules:
//   - Trims whitespace and converts to lowercase
//   - If input contains no dot, appends ".com"
//   - If input contains a dot, uses as-is (preserves non-.com TLDs)
//   - Validates the result is a plausible domain format
//
// Examples:
//   - "EXAMPLE"        → Domain{Full: "example.com", Name: "example", TLD: "com"}
//   - "foo.io"         → Domain{Full: "foo.io", Name: "foo", TLD: "io"}
//   - "example.org"    → Domain{Full: "example.org", Name: "example", TLD: "org"}
//   - "sub.foo.io"     → Domain{Full: "sub.foo.io", Name: "sub.foo", TLD: "io"}
//   - "  TruCore  "    → Domain{Full: "trucore.com", Name: "trucore", TLD: "com"}
//   - ""               → ErrEmptyDomain
//   - "example."       → ErrInvalidFormat
//
// This function uses the safer CLI normalization logic (add .com only if no dot)
// instead of the old server logic (add .com if no .com suffix) which incorrectly
// transformed "example.org" into "example.org.com".
func Normalize(input string) (Domain, error) {
	// Trim whitespace and convert to lowercase
	input = strings.ToLower(strings.TrimSpace(input))

	// Check for empty input
	if input == "" {
		return Domain{}, ErrEmptyDomain
	}

	// If no dot present, append .com (this is the safer CLI logic)
	if !strings.Contains(input, ".") {
		input = input + ".com"
	}

	// Validate format: must not end with a dot
	if strings.HasSuffix(input, ".") {
		return Domain{}, ErrInvalidFormat
	}

	// Split by dots to extract TLD (last part)
	parts := strings.Split(input, ".")
	if len(parts) < 2 {
		return Domain{}, ErrInvalidFormat
	}

	// Extract TLD (last part) and name (everything before TLD)
	tld := parts[len(parts)-1]
	if tld == "" {
		return Domain{}, ErrInvalidFormat
	}

	// Name is everything before the TLD
	name := strings.Join(parts[:len(parts)-1], ".")
	if name == "" {
		return Domain{}, ErrInvalidFormat
	}

	return Domain{
		Full: input,
		Name: name,
		TLD:  tld,
	}, nil
}

// NormalizeBatch normalizes multiple domain inputs.
// Returns parallel slices of domains and errors.
// Invalid domains will have their error in the corresponding position.
func NormalizeBatch(inputs []string) ([]Domain, []error) {
	domains := make([]Domain, len(inputs))
	errs := make([]error, len(inputs))

	for i, input := range inputs {
		domains[i], errs[i] = Normalize(input)
	}

	return domains, errs
}
