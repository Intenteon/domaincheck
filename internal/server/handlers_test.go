package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"domaincheck/internal/domain"
)

// testResult matches the JSON output structure from Result.MarshalJSON
type testResult struct {
	Domain    string `json:"domain"`
	Available bool   `json:"available"`
	Error     string `json:"error,omitempty"`
	Source    string `json:"source,omitempty"`
	CheckedAt string `json:"checked_at,omitempty"`
	Duration  int64  `json:"duration_ms,omitempty"`
}

// testCheckResponse matches the JSON output structure from CheckResponse
type testCheckResponse struct {
	Results   []testResult `json:"results"`
	Checked   int          `json:"checked"`
	Available int          `json:"available"`
	Taken     int          `json:"taken"`
	Errors    int          `json:"errors"`
}

func TestCheckDomainsHandler(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		body       string
		wantStatus int
		wantError  bool
	}{
		{
			name:       "valid POST request",
			method:     http.MethodPost,
			body:       `{"domains": ["example", "test"]}`,
			wantStatus: http.StatusOK,
			wantError:  false,
		},
		{
			name:       "empty domains array",
			method:     http.MethodPost,
			body:       `{"domains": []}`,
			wantStatus: http.StatusBadRequest,
			wantError:  true,
		},
		{
			name:       "invalid JSON",
			method:     http.MethodPost,
			body:       `{invalid json}`,
			wantStatus: http.StatusBadRequest,
			wantError:  true,
		},
		{
			name:       "method not allowed",
			method:     http.MethodGet,
			body:       "",
			wantStatus: http.StatusMethodNotAllowed,
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/check", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			CheckDomainsHandler(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("CheckDomainsHandler() status = %v, want %v", w.Code, tt.wantStatus)
			}

			if !tt.wantError && w.Code == http.StatusOK {
				var resp testCheckResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Errorf("Failed to decode response: %v", err)
				}

				// Verify response structure
				if resp.Checked == 0 {
					t.Error("CheckDomainsHandler() Checked should be > 0")
				}

				if len(resp.Results) == 0 {
					t.Error("CheckDomainsHandler() Results should not be empty")
				}

				// Verify counts add up
				total := resp.Available + resp.Taken + resp.Errors
				if total != resp.Checked {
					t.Errorf("CheckDomainsHandler() counts don't add up: %d + %d + %d != %d",
						resp.Available, resp.Taken, resp.Errors, resp.Checked)
				}
			}
		})
	}
}

func TestCheckDomainsHandlerTooManyDomains(t *testing.T) {
	// Create request with 101 domains
	domains := make([]string, 101)
	for i := range domains {
		domains[i] = "example"
	}

	reqBody := domain.CheckRequest{Domains: domains}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/check", strings.NewReader(string(bodyBytes)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	CheckDomainsHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("CheckDomainsHandler() with 101 domains status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestCheckSingleDomainHandler(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantError  bool
	}{
		{
			name:       "valid domain",
			method:     http.MethodGet,
			path:       "/check/example",
			wantStatus: http.StatusOK,
			wantError:  false,
		},
		{
			name:       "domain with TLD",
			method:     http.MethodGet,
			path:       "/check/example.com",
			wantStatus: http.StatusOK,
			wantError:  false,
		},
		{
			name:       "no domain specified",
			method:     http.MethodGet,
			path:       "/check/",
			wantStatus: http.StatusBadRequest,
			wantError:  true,
		},
		{
			name:       "invalid domain format",
			method:     http.MethodGet,
			path:       "/check/example.",
			wantStatus: http.StatusBadRequest,
			wantError:  true,
		},
		{
			name:       "method not allowed",
			method:     http.MethodPost,
			path:       "/check/example",
			wantStatus: http.StatusMethodNotAllowed,
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			CheckSingleDomainHandler(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("CheckSingleDomainHandler() status = %v, want %v", w.Code, tt.wantStatus)
			}

			if !tt.wantError && w.Code == http.StatusOK {
				var result testResult
				if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
					t.Errorf("Failed to decode response: %v", err)
				}

				// Verify result structure
				if result.Domain == "" {
					t.Error("CheckSingleDomainHandler() Domain should not be empty")
				}

				if result.Source == "" {
					t.Error("CheckSingleDomainHandler() Source should not be empty")
				}

			}
		})
	}
}

func TestHealthHandler(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		wantStatus int
	}{
		{
			name:       "GET request",
			method:     http.MethodGet,
			wantStatus: http.StatusOK,
		},
		{
			name:       "POST method not allowed",
			method:     http.MethodPost,
			wantStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/health", nil)
			w := httptest.NewRecorder()

			HealthHandler(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("HealthHandler() status = %v, want %v", w.Code, tt.wantStatus)
			}

			if w.Code == http.StatusOK {
				var resp map[string]string
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Errorf("Failed to decode response: %v", err)
				}

				if resp["status"] != "ok" {
					t.Errorf("HealthHandler() status = %q, want \"ok\"", resp["status"])
				}
			}
		})
	}
}

// TestCheckDomainsHandlerConcurrency verifies concurrent checks work correctly
func TestCheckDomainsHandlerConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create request with 5 domains to test concurrency
	body := `{"domains": ["example1", "example2", "example3", "example4", "example5"]}`

	req := httptest.NewRequest(http.MethodPost, "/check", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	CheckDomainsHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("CheckDomainsHandler() status = %v, want %v", w.Code, http.StatusOK)
	}

	var resp testCheckResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Checked != 5 {
		t.Errorf("CheckDomainsHandler() Checked = %d, want 5", resp.Checked)
	}

	if len(resp.Results) != 5 {
		t.Errorf("CheckDomainsHandler() Results length = %d, want 5", len(resp.Results))
	}

	// Verify each result has required fields
	for i, result := range resp.Results {
		if result.Domain == "" {
			t.Errorf("Result[%d] Domain is empty", i)
		}
		if result.Source == "" {
			t.Errorf("Result[%d] Source is empty", i)
		}
	}
}
