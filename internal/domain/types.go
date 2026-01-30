// Package domain provides core domain-related types and operations.
package domain

import (
	"encoding/json"
	"time"
)

// Domain represents a validated, normalized domain name.
// It contains both the full domain string and its parsed components.
type Domain struct {
	// Full is the complete normalized domain (e.g., "example.com")
	Full string

	// Name is the second-level domain (e.g., "example")
	Name string

	// TLD is the top-level domain (e.g., "com")
	TLD string
}

// Status represents the availability status of a domain.
type Status int

const (
	// StatusUnknown indicates the status could not be determined
	StatusUnknown Status = iota

	// StatusAvailable indicates the domain is available for registration
	StatusAvailable

	// StatusTaken indicates the domain is already registered
	StatusTaken

	// StatusError indicates an error occurred during checking
	StatusError
)

// String returns the string representation of the Status.
func (s Status) String() string {
	switch s {
	case StatusAvailable:
		return "available"
	case StatusTaken:
		return "taken"
	case StatusError:
		return "error"
	default:
		return "unknown"
	}
}

// Result contains the complete check result for a domain.
// It maintains backward compatibility with the existing API while
// adding new fields for enhanced functionality.
type Result struct {
	// Domain is the normalized domain that was checked
	Domain Domain

	// Status indicates the availability status
	Status Status

	// Available is a convenience field (true when Status == StatusAvailable)
	Available bool

	// Error contains error details when Status == StatusError
	Error string

	// Source indicates which protocol provided the answer (rdap, whois, dns)
	Source string

	// CheckedAt is when the check was performed
	CheckedAt time.Time

	// Duration is how long the check took
	Duration time.Duration
}

// MarshalJSON implements custom JSON marshaling for Result.
// This ensures backward compatibility by:
// - Outputting Domain.Full as a simple "domain" string field
// - Formatting Duration as milliseconds instead of nanoseconds
// - Using existing field names from the original API
func (r Result) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Domain    string `json:"domain"`
		Available bool   `json:"available"`
		Error     string `json:"error,omitempty"`
		Source    string `json:"source,omitempty"`
		CheckedAt string `json:"checked_at,omitempty"`
		Duration  int64  `json:"duration_ms,omitempty"`
	}{
		Domain:    r.Domain.Full,
		Available: r.Available,
		Error:     r.Error,
		Source:    r.Source,
		CheckedAt: r.CheckedAt.Format(time.RFC3339),
		Duration:  r.Duration.Milliseconds(),
	})
}

// CheckRequest represents the JSON body for bulk domain checking requests.
type CheckRequest struct {
	Domains []string `json:"domains"`
}

// CheckResponse represents the JSON response for bulk domain checking.
type CheckResponse struct {
	Results   []Result `json:"results"`
	Checked   int      `json:"checked"`
	Available int      `json:"available"`
	Taken     int      `json:"taken"`
	Errors    int      `json:"errors"`
}
