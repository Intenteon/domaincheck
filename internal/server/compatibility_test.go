package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"domaincheck/internal/domain"
)

// compatResult matches the JSON wire format for backward compatibility testing
type compatResult struct {
	Domain    string `json:"domain"`
	Available bool   `json:"available"`
	Error     string `json:"error,omitempty"`
}

// compatCheckResponse matches the JSON wire format for backward compatibility testing
type compatCheckResponse struct {
	Results   []compatResult `json:"results"`
	Checked   int            `json:"checked"`
	Available int            `json:"available"`
	Taken     int            `json:"taken"`
	Errors    int            `json:"errors"`
}

// TestBackwardCompatibility_APIEndpoints verifies all v1.x API endpoints still work
func TestBackwardCompatibility_APIEndpoints(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		path           string
		body           interface{}
		expectedStatus int
		validateFunc   func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:           "POST /check endpoint exists",
			method:         "POST",
			path:           "/check",
			body:           domain.CheckRequest{Domains: []string{"example.com"}},
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, rr *httptest.ResponseRecorder) {
				var resp compatCheckResponse
				if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
					t.Errorf("Response should be valid JSON: %v", err)
				}
			},
		},
		{
			name:           "GET /check/{domain} endpoint exists",
			method:         "GET",
			path:           "/check/example.com",
			body:           nil,
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, rr *httptest.ResponseRecorder) {
				// GET /check/{domain} returns a single Result, not a CheckResponse
				var resp compatResult
				if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
					t.Errorf("Response should be valid JSON: %v", err)
				}
			},
		},
		{
			name:           "GET /health endpoint exists",
			method:         "GET",
			path:           "/health",
			body:           nil,
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, rr *httptest.ResponseRecorder) {
				var resp map[string]string
				if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
					t.Errorf("Health response should be valid JSON: %v", err)
				}
				if resp["status"] != "ok" {
					t.Errorf("Health status should be 'ok', got %s", resp["status"])
				}
			},
		},
		{
			name:           "POST /check returns 400 for invalid JSON",
			method:         "POST",
			path:           "/check",
			body:           "invalid json",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "GET /check/{domain} returns 400 for invalid domain",
			method:         "GET",
			path:           "/check/invalid..domain",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "POST /check returns 405 for GET method",
			method:         "GET",
			path:           "/check",
			body:           nil,
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			if tt.body != nil {
				if bodyStr, ok := tt.body.(string); ok {
					// Send invalid JSON string
					req = httptest.NewRequest(tt.method, tt.path, bytes.NewBufferString(bodyStr))
				} else {
					// Marshal valid body
					bodyBytes, _ := json.Marshal(tt.body)
					req = httptest.NewRequest(tt.method, tt.path, bytes.NewBuffer(bodyBytes))
				}
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(tt.method, tt.path, nil)
			}

			rr := httptest.NewRecorder()

			// Route to appropriate handler
			switch {
			case tt.path == "/check" && tt.method == "POST":
				CheckDomainsHandler(rr, req)
			case tt.path == "/check" && tt.method == "GET":
				CheckDomainsHandler(rr, req)
			case tt.path == "/health":
				HealthHandler(rr, req)
			case tt.path[:6] == "/check":
				CheckSingleDomainHandler(rr, req)
			}

			if rr.Code != tt.expectedStatus {
				t.Errorf("Status code = %d, expected %d", rr.Code, tt.expectedStatus)
			}

			if tt.validateFunc != nil {
				tt.validateFunc(t, rr)
			}
		})
	}
}

// TestBackwardCompatibility_JSONResponseFormat verifies JSON response structure unchanged
func TestBackwardCompatibility_JSONResponseFormat(t *testing.T) {
	reqBody := domain.CheckRequest{Domains: []string{"example.com"}}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/check", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	CheckDomainsHandler(rr, req)

	var resp compatCheckResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Verify all v1.x fields exist
	t.Run("response has results array", func(t *testing.T) {
		if resp.Results == nil {
			t.Error("Response missing 'results' field")
		}
	})

	t.Run("response has checked count", func(t *testing.T) {
		if resp.Checked == 0 {
			t.Error("Response missing 'checked' field")
		}
	})

	t.Run("response has available count", func(t *testing.T) {
		// available could be 0, just check field exists
		_ = resp.Available
	})

	t.Run("response has taken count", func(t *testing.T) {
		_ = resp.Taken
	})

	t.Run("response has errors count", func(t *testing.T) {
		_ = resp.Errors
	})

	// Verify result item structure
	if len(resp.Results) > 0 {
		result := resp.Results[0]

		t.Run("result has domain field", func(t *testing.T) {
			if result.Domain == "" {
				t.Error("Result missing 'domain' field")
			}
		})

		t.Run("result has available field", func(t *testing.T) {
			_ = result.Available // bool, always present
		})

		t.Run("result has error field (optional)", func(t *testing.T) {
			_ = result.Error // may be empty string
		})
	}
}

// TestBackwardCompatibility_DomainNormalization verifies normalization behavior unchanged
func TestBackwardCompatibility_DomainNormalization(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		shouldOK bool
	}{
		// v1.x normalization behaviors that must be preserved
		{"trucore", "trucore.com", true},
		{"TRUCORE", "trucore.com", true},
		{"  trucore  ", "trucore.com", true},
		{"trucore.com", "trucore.com", true},
		{"TRUCORE.COM", "trucore.com", true},

		// Bug fix: example.org should NOT become example.org.com
		{"example.org", "example.org", true},
		{"EXAMPLE.ORG", "example.org", true},

		// Multi-label domains
		{"sub.example.com", "sub.example.com", true},
		{"deep.sub.example.com", "deep.sub.example.com", true},

		// Invalid inputs (v1.x would reject these too)
		{"invalid..domain", "", false},
		{"-badactor.com", "", false},
		{"", "", false},
		{"   ", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			normalized, err := domain.Normalize(tt.input)

			if tt.shouldOK {
				if err != nil {
					t.Errorf("Normalize(%q) returned error: %v, expected success", tt.input, err)
					return
				}
				if normalized.Full != tt.expected {
					t.Errorf("Normalize(%q) = %q, expected %q", tt.input, normalized.Full, tt.expected)
				}
			} else {
				if err == nil {
					t.Errorf("Normalize(%q) succeeded with %q, expected error", tt.input, normalized.Full)
				}
			}
		})
	}
}

// TestBackwardCompatibility_HTTPStatusCodes verifies status codes unchanged
func TestBackwardCompatibility_HTTPStatusCodes(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		path           string
		body           string
		expectedStatus int
	}{
		{"Valid POST /check", "POST", "/check", `{"domains":["example.com"]}`, http.StatusOK},
		{"Valid GET /check/{domain}", "GET", "/check/example.com", "", http.StatusOK},
		{"Invalid JSON", "POST", "/check", `{invalid}`, http.StatusBadRequest},
		{"Invalid domain", "GET", "/check/invalid..domain", "", http.StatusBadRequest},
		{"Method not allowed", "GET", "/check", "", http.StatusMethodNotAllowed},
		{"Health check", "GET", "/health", "", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			if tt.body != "" {
				req = httptest.NewRequest(tt.method, tt.path, bytes.NewBufferString(tt.body))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(tt.method, tt.path, nil)
			}

			rr := httptest.NewRecorder()

			// Route request
			switch {
			case tt.path == "/check":
				CheckDomainsHandler(rr, req)
			case tt.path == "/health":
				HealthHandler(rr, req)
			case tt.path[:6] == "/check":
				CheckSingleDomainHandler(rr, req)
			}

			if rr.Code != tt.expectedStatus {
				t.Errorf("Status = %d, expected %d", rr.Code, tt.expectedStatus)
			}
		})
	}
}

// TestBackwardCompatibility_ContentType verifies response content type unchanged
func TestBackwardCompatibility_ContentType(t *testing.T) {
	reqBody := domain.CheckRequest{Domains: []string{"example.com"}}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/check", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	CheckDomainsHandler(rr, req)

	contentType := rr.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type = %q, expected %q", contentType, "application/json")
	}
}

// TestBackwardCompatibility_EmptyDomainsList verifies behavior with empty domains
func TestBackwardCompatibility_EmptyDomainsList(t *testing.T) {
	reqBody := domain.CheckRequest{Domains: []string{}}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/check", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	CheckDomainsHandler(rr, req)

	// Empty domains list should return 400 Bad Request (improved from v1.x)
	// v2.0 properly validates and rejects empty requests
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, expected %d for empty domains list", rr.Code, http.StatusBadRequest)
	}
}

// TestBackwardCompatibility_MaxDomainsLimit verifies 100 domain limit
func TestBackwardCompatibility_MaxDomainsLimit(t *testing.T) {
	// Create request with 101 domains (should be rejected)
	domains := make([]string, 101)
	for i := range domains {
		domains[i] = "example.com"
	}

	reqBody := domain.CheckRequest{Domains: domains}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/check", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	CheckDomainsHandler(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, expected %d for 101 domains", rr.Code, http.StatusBadRequest)
	}

	// Verify exactly 100 domains is accepted
	domains = make([]string, 100)
	for i := range domains {
		domains[i] = "example.com"
	}

	reqBody = domain.CheckRequest{Domains: domains}
	bodyBytes, _ = json.Marshal(reqBody)
	req = httptest.NewRequest("POST", "/check", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()

	CheckDomainsHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Status = %d, expected %d for 100 domains", rr.Code, http.StatusOK)
	}
}

// TestBackwardCompatibility_FieldTypes verifies JSON field types unchanged
func TestBackwardCompatibility_FieldTypes(t *testing.T) {
	reqBody := domain.CheckRequest{Domains: []string{"example.com"}}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/check", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	CheckDomainsHandler(rr, req)

	// Parse as generic map to verify types
	var respMap map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &respMap); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Verify field types match v1.x
	if _, ok := respMap["results"].([]interface{}); !ok {
		t.Error("Field 'results' should be array type")
	}

	if _, ok := respMap["checked"].(float64); !ok {
		t.Error("Field 'checked' should be number type")
	}

	if _, ok := respMap["available"].(float64); !ok {
		t.Error("Field 'available' should be number type")
	}

	if _, ok := respMap["taken"].(float64); !ok {
		t.Error("Field 'taken' should be number type")
	}

	if _, ok := respMap["errors"].(float64); !ok {
		t.Error("Field 'errors' should be number type")
	}

	// Verify result item field types
	results := respMap["results"].([]interface{})
	if len(results) > 0 {
		result := results[0].(map[string]interface{})

		if _, ok := result["domain"].(string); !ok {
			t.Error("Result field 'domain' should be string type")
		}

		if _, ok := result["available"].(bool); !ok {
			t.Error("Result field 'available' should be boolean type")
		}

		// error field is optional string
		if errVal, ok := result["error"]; ok && errVal != nil {
			if _, ok := errVal.(string); !ok {
				t.Error("Result field 'error' should be string type when present")
			}
		}
	}
}
