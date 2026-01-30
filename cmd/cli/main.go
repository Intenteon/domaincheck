package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const defaultServer = "http://localhost:8765"

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

func usage() {
	fmt.Fprintf(os.Stderr, `Domain Checker CLI

Usage:
  domaincheck <domain>                    Check single domain
  domaincheck <domain1> <domain2> ...     Check multiple domains
  domaincheck -f <file>                   Check domains from file (one per line)
  domaincheck -                           Read domains from stdin

Options:
  -s <server>    Server URL (default: %s)
  -j             Output raw JSON
  -a             Show only available domains
  -q             Quiet mode (exit code only: 0=available, 1=taken/error)
  -h             Show this help

Examples:
  domaincheck trucore.com
  domaincheck trucore priment axient
  domaincheck -f domains.txt
  echo -e "trucore\npriment\naxient" | domaincheck -
  domaincheck -a trucore priment axient   # Only show available

`, defaultServer)
	os.Exit(1)
}

func main() {
	if len(os.Args) < 2 {
		usage()
	}

	server := defaultServer
	jsonOutput := false
	onlyAvailable := false
	quiet := false
	var domains []string
	var inputFile string

	// Parse args
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-h", "--help":
			usage()
		case "-s", "--server":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "Error: -s requires server URL")
				os.Exit(1)
			}
			i++
			server = args[i]
		case "-j", "--json":
			jsonOutput = true
		case "-a", "--available":
			onlyAvailable = true
		case "-q", "--quiet":
			quiet = true
		case "-f", "--file":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "Error: -f requires filename")
				os.Exit(1)
			}
			i++
			inputFile = args[i]
		case "-":
			// Read from stdin
			inputFile = "-"
		default:
			// Treat as domain
			domains = append(domains, arg)
		}
	}

	// Read domains from file if specified
	if inputFile != "" {
		var reader io.Reader
		if inputFile == "-" {
			reader = os.Stdin
		} else {
			f, err := os.Open(inputFile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error opening file: %v\n", err)
				os.Exit(1)
			}
			defer f.Close()
			reader = f
		}

		data, err := io.ReadAll(reader)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
			os.Exit(1)
		}

		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") {
				domains = append(domains, line)
			}
		}
	}

	if len(domains) == 0 {
		fmt.Fprintln(os.Stderr, "Error: No domains specified")
		os.Exit(1)
	}

	// Normalize domains
	for i, d := range domains {
		d = strings.ToLower(strings.TrimSpace(d))
		if !strings.Contains(d, ".") {
			d = d + ".com"
		}
		domains[i] = d
	}

	// Make request
	reqBody := CheckRequest{Domains: domains}
	jsonData, _ := json.Marshal(reqBody)

	client := &http.Client{Timeout: time.Duration(len(domains)*12) * time.Second}
	resp, err := client.Post(server+"/check", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to server: %v\n", err)
		fmt.Fprintln(os.Stderr, "Make sure the server is running: go run cmd/server/main.go")
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(os.Stderr, "Server error (%d): %s\n", resp.StatusCode, string(body))
		os.Exit(1)
	}

	var result CheckResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing response: %v\n", err)
		os.Exit(1)
	}

	// Output results
	if quiet {
		// Exit code based on first domain availability
		if len(result.Results) > 0 && result.Results[0].Available {
			os.Exit(0)
		}
		os.Exit(1)
	}

	if jsonOutput {
		if onlyAvailable {
			filtered := CheckResponse{
				Results: []DomainResult{},
			}
			for _, r := range result.Results {
				if r.Available {
					filtered.Results = append(filtered.Results, r)
					filtered.Available++
				}
			}
			filtered.Checked = len(filtered.Results)
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			enc.Encode(filtered)
		} else {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			enc.Encode(result)
		}
		return
	}

	// Pretty print
	for _, r := range result.Results {
		if onlyAvailable && !r.Available {
			continue
		}

		if r.Available {
			fmt.Printf("✓ %-30s AVAILABLE\n", r.Domain)
		} else if r.Error != "" {
			fmt.Printf("? %-30s ERROR: %s\n", r.Domain, r.Error)
		} else {
			fmt.Printf("✗ %-30s TAKEN\n", r.Domain)
		}
	}

	if !onlyAvailable {
		fmt.Printf("\n--- Summary ---\n")
		fmt.Printf("Checked: %d | Available: %d | Taken: %d | Errors: %d\n",
			result.Checked, result.Available, result.Taken, result.Errors)
	}

	// Exit with error if no domains available
	if result.Available == 0 {
		os.Exit(1)
	}
}
