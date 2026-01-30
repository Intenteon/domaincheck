package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const (
	maxDomainsPerRequest = 100
	whoisTimeout         = 10 * time.Second
	maxConcurrent        = 10
)

type CheckRequest struct {
	Domains []string `json:"domains"`
}

type DomainResult struct {
	Domain    string `json:"domain"`
	Available bool   `json:"available"`
	Error     string `json:"error,omitempty"`
}

type CheckResponse struct {
	Results   []DomainResult `json:"results"`
	Checked   int            `json:"checked"`
	Available int            `json:"available"`
	Taken     int            `json:"taken"`
	Errors    int            `json:"errors"`
}

func checkDomain(domain string) DomainResult {
	// Normalize domain
	domain = strings.ToLower(strings.TrimSpace(domain))
	if !strings.HasSuffix(domain, ".com") {
		domain = domain + ".com"
	}

	ctx, cancel := context.WithTimeout(context.Background(), whoisTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "whois", domain)
	output, err := cmd.CombinedOutput()

	result := DomainResult{Domain: domain}

	if ctx.Err() == context.DeadlineExceeded {
		result.Error = "timeout"
		return result
	}

	if err != nil {
		// Some whois errors are expected for available domains
		outputStr := string(output)
		if strings.Contains(outputStr, "No match") ||
			strings.Contains(outputStr, "NOT FOUND") ||
			strings.Contains(outputStr, "No entries found") ||
			strings.Contains(outputStr, "no matching record") {
			result.Available = true
			return result
		}
		result.Error = fmt.Sprintf("whois error: %v", err)
		return result
	}

	outputStr := string(output)

	// Check for availability indicators
	availableIndicators := []string{
		"No match for domain",
		"No match for \"",
		"NOT FOUND",
		"No entries found",
		"no matching record",
		"Domain not found",
		"No Data Found",
		"Status: AVAILABLE",
	}

	for _, indicator := range availableIndicators {
		if strings.Contains(outputStr, indicator) {
			result.Available = true
			return result
		}
	}

	// Check for taken indicators
	takenIndicators := []string{
		"Registry Domain ID:",
		"Creation Date:",
		"Registrar:",
		"Domain Status:",
		"Name Server:",
	}

	for _, indicator := range takenIndicators {
		if strings.Contains(outputStr, indicator) {
			result.Available = false
			return result
		}
	}

	// Default to unknown/error if we can't determine
	result.Error = "unable to determine availability"
	return result
}

func checkDomainsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	if len(req.Domains) == 0 {
		http.Error(w, "No domains provided", http.StatusBadRequest)
		return
	}

	if len(req.Domains) > maxDomainsPerRequest {
		http.Error(w, fmt.Sprintf("Maximum %d domains per request", maxDomainsPerRequest), http.StatusBadRequest)
		return
	}

	// Process domains concurrently with a semaphore
	results := make([]DomainResult, len(req.Domains))
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrent)

	for i, domain := range req.Domains {
		wg.Add(1)
		go func(idx int, d string) {
			defer wg.Done()
			sem <- struct{}{}        // Acquire
			defer func() { <-sem }() // Release

			results[idx] = checkDomain(d)
		}(i, domain)
	}

	wg.Wait()

	// Build response
	response := CheckResponse{
		Results: results,
		Checked: len(results),
	}

	for _, r := range results {
		if r.Error != "" {
			response.Errors++
		} else if r.Available {
			response.Available++
		} else {
			response.Taken++
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8765"
	}

	http.HandleFunc("/check", checkDomainsHandler)
	http.HandleFunc("/health", healthHandler)

	// Also support single domain via query param for convenience
	http.HandleFunc("/check/", func(w http.ResponseWriter, r *http.Request) {
		domain := strings.TrimPrefix(r.URL.Path, "/check/")
		if domain == "" {
			http.Error(w, "No domain specified", http.StatusBadRequest)
			return
		}

		result := checkDomain(domain)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	log.Printf("Domain checker service starting on port %s", port)
	log.Printf("Endpoints:")
	log.Printf("  POST /check         - Check multiple domains (JSON body: {\"domains\": [...]})")
	log.Printf("  GET  /check/{domain} - Check single domain")
	log.Printf("  GET  /health        - Health check")

	// Read from stdin for interactive mode
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		fmt.Println("\nInteractive mode: Enter domains to check (one per line, 'quit' to exit)")
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "quit" || line == "exit" {
				os.Exit(0)
			}
			if line == "" {
				continue
			}
			result := checkDomain(line)
			if result.Available {
				fmt.Printf("✓ %s - AVAILABLE\n", result.Domain)
			} else if result.Error != "" {
				fmt.Printf("? %s - ERROR: %s\n", result.Domain, result.Error)
			} else {
				fmt.Printf("✗ %s - TAKEN\n", result.Domain)
			}
		}
	}()

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
