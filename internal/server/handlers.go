// Package server provides HTTP handlers for the domain checking API.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"domaincheck/internal/checker"
	"domaincheck/internal/domain"
)

const (
	// maxDomainsPerRequest limits bulk domain checks to prevent abuse
	maxDomainsPerRequest = 100

	// maxConcurrent limits parallel domain checks to prevent resource exhaustion
	maxConcurrent = 10

	// requestTimeout is the maximum time allowed for a bulk check request
	requestTimeout = 60 * time.Second
)

// CheckDomainsHandler handles POST /check for bulk domain availability checking.
//
// Request Body:
//
//	{
//	  "domains": ["example.com", "test.org", ...]
//	}
//
// Response:
//
//	{
//	  "results": [
//	    {"domain": "example.com", "available": false, "source": "dns", ...},
//	    ...
//	  ],
//	  "checked": 2,
//	  "available": 1,
//	  "taken": 1,
//	  "errors": 0
//	}
//
// The handler:
//   - Validates the request (max 100 domains)
//   - Normalizes domain inputs
//   - Checks domains concurrently (max 10 parallel)
//   - Returns aggregated results with counts
func CheckDomainsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// SECURITY: Limit request body to 1MB to prevent DoS via large payloads
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()

	// Parse request body
	var req domain.CheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Validate request
	if len(req.Domains) == 0 {
		http.Error(w, "No domains provided", http.StatusBadRequest)
		return
	}

	if len(req.Domains) > maxDomainsPerRequest {
		http.Error(w, fmt.Sprintf("Maximum %d domains per request", maxDomainsPerRequest), http.StatusBadRequest)
		return
	}

	// SECURITY: Add explicit request timeout to prevent long-running requests
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	// Normalize domains first
	normalizedDomains := make([]domain.Domain, len(req.Domains))
	normErrors := make([]error, len(req.Domains))
	for i, input := range req.Domains {
		normalizedDomains[i], normErrors[i] = domain.Normalize(input)
	}

	// Check domains concurrently with semaphore
	results := make([]domain.Result, len(req.Domains))
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrent)

	for i := range req.Domains {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			// SECURITY: Respect context cancellation at semaphore acquisition
			select {
			case sem <- struct{}{}: // Acquire
				defer func() { <-sem }() // Release
			case <-ctx.Done():
				// Context cancelled while waiting for semaphore
				results[idx] = domain.Result{
					Domain:    normalizedDomains[idx],
					Status:    domain.StatusError,
					Available: false,
					Error:     "request cancelled",
				}
				return
			}

			// If normalization failed, create error result
			if normErrors[idx] != nil {
				results[idx] = domain.Result{
					Domain:    domain.Domain{Full: req.Domains[idx]},
					Status:    domain.StatusError,
					Available: false,
					Error:     "invalid domain format",
				}
				return
			}

			// Perform the check with request context
			result, err := checker.Check(ctx, normalizedDomains[idx])
			if err != nil {
				// Check failed - result already has error info
				results[idx] = result
				return
			}

			results[idx] = result
		}(i)
	}

	wg.Wait()

	// Build response with counts
	response := domain.CheckResponse{
		Results: results,
		Checked: len(results),
	}

	for _, res := range results {
		if res.Error != "" {
			response.Errors++
		} else if res.Available {
			response.Available++
		} else {
			response.Taken++
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		// Log encoding error (headers already sent, can't change status)
		log.Printf("Failed to encode response: %v", err)
	}
}

// CheckSingleDomainHandler handles GET /check/{domain} for single domain checks.
//
// URL: /check/example.com
//
// Response:
//
//	{
//	  "domain": "example.com",
//	  "available": false,
//	  "source": "dns",
//	  ...
//	}
//
// The handler:
//   - Extracts domain from URL path
//   - Normalizes the domain
//   - Performs availability check
//   - Returns single result
func CheckSingleDomainHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract domain from URL path (/check/{domain})
	path := strings.TrimPrefix(r.URL.Path, "/check/")
	if path == "" || path == r.URL.Path {
		http.Error(w, "No domain specified", http.StatusBadRequest)
		return
	}

	// Normalize domain
	d, err := domain.Normalize(path)
	if err != nil {
		http.Error(w, "Invalid domain format", http.StatusBadRequest)
		return
	}

	// Perform check
	result, err := checker.Check(r.Context(), d)
	if err != nil {
		// Result already contains error info
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK) // Return 200 with error in result
		if encErr := json.NewEncoder(w).Encode(result); encErr != nil {
			log.Printf("Failed to encode error response: %v", encErr)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

// HealthHandler handles GET /health for health checks.
//
// Response:
//
//	{
//	  "status": "ok"
//	}
//
// Always returns 200 OK if the server is running.
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
		log.Printf("Failed to encode health response: %v", err)
	}
}
