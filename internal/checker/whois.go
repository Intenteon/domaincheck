package checker

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"domaincheck/internal/domain"
)

// WHOISCheck queries the whois system to check if a domain is registered.
//
// This is a fallback mechanism used when RDAP is unavailable or doesn't support the TLD.
// WHOIS is the legacy protocol that predates RDAP and has inconsistent output formats.
//
// The function:
//   - Executes the system `whois` command
//   - Parses unstructured text output to determine availability
//   - Returns availability based on pattern matching
//
// Important Notes:
//   - Requires `whois` command to be installed on the system
//   - Output format varies by TLD and WHOIS server
//   - Less reliable than RDAP due to inconsistent formats
//   - Exit code 1 is sometimes returned for available domains (not an error)
//
// Returns:
//   - available: true if domain is available for registration
//   - err: error if whois command failed or could not be executed
func WHOISCheck(ctx context.Context, d domain.Domain) (available bool, err error) {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Execute whois command
	cmd := exec.CommandContext(ctx, "whois", d.Full)
	output, cmdErr := cmd.CombinedOutput()
	outputStr := string(output)

	// Check for availability indicators first (even if command errored)
	// WHOIS servers often return exit code 1 for available domains
	availableIndicators := []string{
		"No match for domain",
		"No match for \"",
		"NOT FOUND",
		"No entries found",
		"no matching record",
		"Domain not found",
		"No Data Found",
		"Status: AVAILABLE",
		"Not found:",
	}

	for _, indicator := range availableIndicators {
		if strings.Contains(outputStr, indicator) {
			return true, nil
		}
	}

	// Check for taken indicators
	takenIndicators := []string{
		"Registry Domain ID:",
		"Creation Date:",
		"Registrar:",
		"Domain Status:",
		"Name Server:",
		"Registrant Name:",
	}

	for _, indicator := range takenIndicators {
		if strings.Contains(outputStr, indicator) {
			return false, nil
		}
	}

	// If command had a real error (not just exit code 1), handle it
	if cmdErr != nil {
		// Context deadline exceeded
		if ctx.Err() == context.DeadlineExceeded {
			return false, fmt.Errorf("whois timeout: %w", ctx.Err())
		}

		// Exit code 1 is often normal for available domains
		if exitErr, ok := cmdErr.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 1 && len(output) > 0 {
				// Had output but no clear indicators - assume taken to be safe
				return false, nil
			}
		}

		// Other errors (command not found, etc.)
		return false, fmt.Errorf("whois command failed: %w", cmdErr)
	}

	// If we have output but couldn't determine status, assume taken to be safe
	if len(output) > 0 {
		return false, nil
	}

	// No output and no error is unusual - return error
	return false, fmt.Errorf("whois returned no output")
}
