package urlclass

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ilpy20/langid-go"
)

func TestURLClassifyAndRank(t *testing.T) {
	// Initialize the default identifier for testing
	id, err := langid.NewDefaultIdentifier()
	if err != nil {
		t.Fatalf("failed to load default identifier: %v", err)
	}

	client := NewClient(id)

	// Distinct German phrase to ensure consistent Naive Bayes classification as "de"
	testContent := "Guten Tag, wie geht es Ihnen? Alles ist wunderbar hier."
	expectedLength := len(testContent)

	// Create a test server that returns German content
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(testContent))
	}))
	defer server.Close()

	t.Run("ClassifyURL Success", func(t *testing.T) {
		res, length, err := client.ClassifyURL(server.URL, 5*time.Second)
		if err != nil {
			t.Fatalf("unexpected error in ClassifyURL: %v", err)
		}
		if length != expectedLength {
			t.Errorf("expected length %d, got %d", expectedLength, length)
		}
		if res.Language != "de" {
			t.Errorf("expected language 'de', got %q", res.Language)
		}
	})

	t.Run("RankURL Success", func(t *testing.T) {
		results, length, err := client.RankURL(server.URL, 5*time.Second)
		if err != nil {
			t.Fatalf("unexpected error in RankURL: %v", err)
		}
		if length != expectedLength {
			t.Errorf("expected length %d, got %d", expectedLength, length)
		}
		if len(results) == 0 {
			t.Fatal("expected ranked results, got empty slice")
		}
		if results[0].Language != "de" {
			t.Errorf("expected top ranked language 'de', got %q", results[0].Language)
		}
	})
}

func TestURLClassifyErrors(t *testing.T) {
	id, err := langid.NewDefaultIdentifier()
	if err != nil {
		t.Fatalf("failed to load default identifier: %v", err)
	}

	client := NewClient(id)

	t.Run("HTTP Non-200 Status", func(t *testing.T) {
		// Test server returning a 404 status
		errServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer errServer.Close()

		_, _, err := client.ClassifyURL(errServer.URL, 5*time.Second)
		if err == nil {
			t.Fatal("expected error for non-200 status, got nil")
		}
		if !strings.Contains(err.Error(), "status code 404") {
			t.Errorf("expected error containing 'status code 404', got %q", err.Error())
		}
	})

	t.Run("Unreachable Host", func(t *testing.T) {
		// Connect to an invalid address/port that is guaranteed to fail
		_, _, err := client.ClassifyURL("http://invalid.local-unreachable-domain.xyz", 1*time.Second)
		if err == nil {
			t.Fatal("expected error for unreachable host, got nil")
		}
	})

	t.Run("Timeout Behavior", func(t *testing.T) {
		// Create a slow server that delays response beyond timeout limit
		slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer slowServer.Close()

		// Request with extremely short timeout of 10ms
		_, _, err := client.ClassifyURL(slowServer.URL, 10*time.Millisecond)
		if err == nil {
			t.Fatal("expected timeout error, got nil")
		}
		if !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "Client.Timeout exceeded") && !strings.Contains(err.Error(), "deadline exceeded") {
			t.Errorf("expected timeout/deadline exceeded error, got: %v", err)
		}
	})
}
