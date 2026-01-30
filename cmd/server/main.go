package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"domaincheck/internal/checker"
	"domaincheck/internal/domain"
	"domaincheck/internal/server"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8765"
	}

	// Register HTTP handlers from internal/server package
	http.HandleFunc("/check", server.CheckDomainsHandler)
	http.HandleFunc("/check/", server.CheckSingleDomainHandler)
	http.HandleFunc("/health", server.HealthHandler)

	log.Printf("Domain checker service starting on port %s", port)
	log.Printf("Endpoints:")
	log.Printf("  POST /check         - Check multiple domains (JSON body: {\"domains\": [...]})")
	log.Printf("  GET  /check/{domain} - Check single domain")
	log.Printf("  GET  /health        - Health check")

	// Interactive mode: Read from stdin for convenience
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		fmt.Println("\nInteractive mode: Enter domains to check (one per line, 'quit' to exit)")
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "quit" || line == "exit" {
				log.Println("Exiting interactive mode")
				os.Exit(0)
			}
			if line == "" {
				continue
			}

			// Normalize and check domain
			d, err := domain.Normalize(line)
			if err != nil {
				fmt.Printf("? %s - ERROR: invalid domain format\n", line)
				continue
			}

			result, err := checker.Check(context.Background(), d)
			if err != nil || result.Error != "" {
				errorMsg := result.Error
				if err != nil {
					errorMsg = err.Error()
				}
				fmt.Printf("? %s - ERROR: %s\n", result.Domain.Full, errorMsg)
			} else if result.Available {
				fmt.Printf("✓ %s - AVAILABLE (via %s)\n", result.Domain.Full, result.Source)
			} else {
				fmt.Printf("✗ %s - TAKEN (via %s)\n", result.Domain.Full, result.Source)
			}
		}
	}()

	// Start HTTP server
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
