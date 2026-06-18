package service_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ilpy20/langid-go"
	"github.com/ilpy20/langid-go/service"
)

func TestServiceHandlers(t *testing.T) {
	id, err := langid.NewDefaultIdentifier()
	if err != nil {
		t.Fatalf("failed to load default identifier: %v", err)
	}

	srv := service.NewServer(id)
	handler := srv.NewHandler()

	type responseEnvelope struct {
		ResponseData    any     `json:"responseData"`
		ResponseDetails *string `json:"responseDetails"`
		ResponseStatus  int     `json:"responseStatus"`
	}

	tests := []struct {
		name           string
		method         string
		path           string
		body           string
		contentType    string
		expectedStatus int
		verify         func(t *testing.T, resp responseEnvelope)
	}{
		{
			name:           "Detect GET with q",
			method:         "GET",
			path:           "/detect?q=hello+world",
			expectedStatus: http.StatusOK,
			verify: func(t *testing.T, resp responseEnvelope) {
				if resp.ResponseData == nil {
					t.Errorf("expected non-nil responseData")
					return
				}
				dataMap, ok := resp.ResponseData.(map[string]any)
				if !ok {
					t.Errorf("expected responseData to be a map, got %T", resp.ResponseData)
					return
				}
				lang, ok := dataMap["language"].(string)
				if !ok || lang != "en" {
					t.Errorf("expected language 'en', got %v", dataMap["language"])
				}
				conf, ok := dataMap["confidence"].(float64)
				if !ok || conf <= 0.0 || conf > 1.0 {
					t.Errorf("expected confidence between 0 and 1, got %v", dataMap["confidence"])
				}
			},
		},
		{
			name:           "Detect GET with missing q",
			method:         "GET",
			path:           "/detect",
			expectedStatus: http.StatusOK,
			verify: func(t *testing.T, resp responseEnvelope) {
				if resp.ResponseData != nil {
					t.Errorf("expected nil responseData for missing q, got %v", resp.ResponseData)
				}
			},
		},
		{
			name:           "Detect POST form with q",
			method:         "POST",
			path:           "/detect",
			body:           "q=Guten+Tag,+wie+geht+es+Ihnen%3F",
			contentType:    "application/x-www-form-urlencoded",
			expectedStatus: http.StatusOK,
			verify: func(t *testing.T, resp responseEnvelope) {
				dataMap, ok := resp.ResponseData.(map[string]any)
				if !ok {
					t.Fatalf("expected map response")
				}
				if dataMap["language"] != "de" {
					t.Errorf("expected language 'de', got %v", dataMap["language"])
				}
			},
		},
		{
			name:           "Detect POST raw body",
			method:         "POST",
			path:           "/detect",
			body:           "Bonjour tout le monde",
			contentType:    "text/plain",
			expectedStatus: http.StatusOK,
			verify: func(t *testing.T, resp responseEnvelope) {
				dataMap, ok := resp.ResponseData.(map[string]any)
				if !ok {
					t.Fatalf("expected map response")
				}
				if dataMap["language"] != "fr" {
					t.Errorf("expected language 'fr', got %v", dataMap["language"])
				}
			},
		},
		{
			name:           "Detect PUT raw body",
			method:         "PUT",
			path:           "/detect",
			body:           "This is an english test sentence.",
			expectedStatus: http.StatusOK,
			verify: func(t *testing.T, resp responseEnvelope) {
				dataMap, ok := resp.ResponseData.(map[string]any)
				if !ok {
					t.Fatalf("expected map response")
				}
				if dataMap["language"] != "en" {
					t.Errorf("expected language 'en', got %v", dataMap["language"])
				}
			},
		},
		{
			name:           "Rank GET with q",
			method:         "GET",
			path:           "/rank?q=hello+world",
			expectedStatus: http.StatusOK,
			verify: func(t *testing.T, resp responseEnvelope) {
				slice, ok := resp.ResponseData.([]any)
				if !ok {
					t.Fatalf("expected slice response for /rank, got %T", resp.ResponseData)
				}
				if len(slice) == 0 {
					t.Fatalf("expected non-empty rank list")
				}
				first, ok := slice[0].([]any)
				if !ok || len(first) < 2 {
					t.Fatalf("expected list of pairs, got %v", slice[0])
				}
				if first[0] != "en" {
					t.Errorf("expected top language 'en', got %v", first[0])
				}
			},
		},
		{
			name:           "Rank GET with missing q",
			method:         "GET",
			path:           "/rank",
			expectedStatus: http.StatusOK,
			verify: func(t *testing.T, resp responseEnvelope) {
				if resp.ResponseData != nil {
					t.Errorf("expected nil responseData for missing q, got %v", resp.ResponseData)
				}
			},
		},
		{
			name:           "Rank POST form with q",
			method:         "POST",
			path:           "/rank",
			body:           "q=Guten+Tag,+wie+geht+es+Ihnen%3F",
			contentType:    "application/x-www-form-urlencoded",
			expectedStatus: http.StatusOK,
			verify: func(t *testing.T, resp responseEnvelope) {
				slice, ok := resp.ResponseData.([]any)
				if !ok {
					t.Fatalf("expected slice")
				}
				first := slice[0].([]any)
				if first[0] != "de" {
					t.Errorf("expected top language 'de', got %v", first[0])
				}
			},
		},
		{
			name:           "Rank POST raw body",
			method:         "POST",
			path:           "/rank",
			body:           "Bonjour tout le monde",
			contentType:    "text/plain",
			expectedStatus: http.StatusOK,
			verify: func(t *testing.T, resp responseEnvelope) {
				slice, ok := resp.ResponseData.([]any)
				if !ok {
					t.Fatalf("expected slice")
				}
				first := slice[0].([]any)
				if first[0] != "fr" {
					t.Errorf("expected top language 'fr', got %v", first[0])
				}
			},
		},
		{
			name:           "Rank PUT raw body",
			method:         "PUT",
			path:           "/rank",
			body:           "This is an english test sentence.",
			expectedStatus: http.StatusOK,
			verify: func(t *testing.T, resp responseEnvelope) {
				slice, ok := resp.ResponseData.([]any)
				if !ok {
					t.Fatalf("expected slice")
				}
				first := slice[0].([]any)
				if first[0] != "en" {
					t.Errorf("expected top language 'en', got %v", first[0])
				}
			},
		},
		{
			name:           "Unsupported HTTP Method (DELETE)",
			method:         "DELETE",
			path:           "/detect",
			expectedStatus: http.StatusMethodNotAllowed,
			verify: func(t *testing.T, resp responseEnvelope) {
				if resp.ResponseDetails == nil || !strings.Contains(*resp.ResponseDetails, "DELETE not allowed") {
					t.Errorf("expected 'DELETE not allowed' in responseDetails, got %v", resp.ResponseDetails)
				}
			},
		},
		{
			name:           "Not Found Endpoint",
			method:         "GET",
			path:           "/unsupported_endpoint",
			expectedStatus: http.StatusNotFound,
			verify: func(t *testing.T, resp responseEnvelope) {
				if resp.ResponseDetails == nil || *resp.ResponseDetails != "Not found" {
					t.Errorf("expected 'Not found' in responseDetails, got %v", resp.ResponseDetails)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			if tt.body != "" {
				req = httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			} else {
				req = httptest.NewRequest(tt.method, tt.path, nil)
			}

			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Fatalf("expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			// Validate Content-Type
			contentType := rr.Header().Get("Content-Type")
			if !strings.Contains(contentType, "text/javascript") {
				t.Errorf("expected Content-Type to contain text/javascript, got %q", contentType)
			}

			var envelope responseEnvelope
			if err := json.NewDecoder(rr.Body).Decode(&envelope); err != nil {
				t.Fatalf("failed to decode JSON response: %v", err)
			}

			if envelope.ResponseStatus != tt.expectedStatus {
				t.Errorf("envelope status %d does not match HTTP status %d", envelope.ResponseStatus, tt.expectedStatus)
			}

			tt.verify(t, envelope)
		})
	}
}

func TestDemoHandler(t *testing.T) {
	id, _ := langid.NewDefaultIdentifier()
	srv := service.NewServer(id)
	handler := srv.NewHandler()

	req := httptest.NewRequest("GET", "/demo", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	contentType := rr.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Errorf("expected Content-Type to contain text/html, got %q", contentType)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "<html>") || !strings.Contains(body, "typerArea") {
		t.Errorf("returned body does not look like the demo form HTML")
	}
}

func TestServiceRequestSizeLimit(t *testing.T) {
	id, err := langid.NewDefaultIdentifier()
	if err != nil {
		t.Fatalf("failed to load default identifier: %v", err)
	}

	srv := service.NewServer(id)
	srv.SetMaxRequestBytes(4)
	handler := srv.NewHandler()

	req := httptest.NewRequest(http.MethodPost, "/detect", strings.NewReader("hello"))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status %d, got %d", http.StatusRequestEntityTooLarge, rr.Code)
	}

	var envelope struct {
		ResponseDetails *string `json:"responseDetails"`
		ResponseStatus  int     `json:"responseStatus"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&envelope); err != nil {
		t.Fatalf("failed to decode JSON response: %v", err)
	}
	if envelope.ResponseStatus != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected envelope status %d, got %d", http.StatusRequestEntityTooLarge, envelope.ResponseStatus)
	}
	if envelope.ResponseDetails == nil || !strings.Contains(*envelope.ResponseDetails, "exceeds 4 bytes") {
		t.Fatalf("expected response details to mention the 4-byte limit, got %v", envelope.ResponseDetails)
	}
}
