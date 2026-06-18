package urlclass

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/ilpy20/langid-go"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestURLClassifyAndRank(t *testing.T) {
	id, err := langid.NewDefaultIdentifier()
	if err != nil {
		t.Fatalf("failed to load default identifier: %v", err)
	}

	testContent := "Guten Tag, wie geht es Ihnen? Alles ist wunderbar hier."
	expectedLength := len(testContent)

	httpClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       io.NopCloser(strings.NewReader(testContent)),
				Header:     make(http.Header),
				Request:    req,
			}, nil
		}),
	}
	client := NewClientWithHTTPClient(id, httpClient)

	t.Run("ClassifyURL Success", func(t *testing.T) {
		res, length, err := client.ClassifyURL("https://example.com", 5*time.Second)
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
		results, length, err := client.RankURL("https://example.com", 5*time.Second)
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

	t.Run("HTTP Non-200 Status", func(t *testing.T) {
		client := NewClientWithHTTPClient(id, &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Status:     "404 Not Found",
					Body:       io.NopCloser(strings.NewReader("")),
					Header:     make(http.Header),
					Request:    req,
				}, nil
			}),
		})

		_, _, err := client.ClassifyURL("https://example.com", 5*time.Second)
		if err == nil {
			t.Fatal("expected error for non-200 status, got nil")
		}
		if !strings.Contains(err.Error(), "status code 404") {
			t.Errorf("expected error containing 'status code 404', got %q", err.Error())
		}
	})

	t.Run("Transport Error", func(t *testing.T) {
		client := NewClientWithHTTPClient(id, &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return nil, errors.New("dial failed")
			}),
		})

		_, _, err := client.ClassifyURL("https://example.com", time.Second)
		if err == nil {
			t.Fatal("expected transport error, got nil")
		}
	})

	t.Run("Timeout Behavior", func(t *testing.T) {
		client := NewClientWithHTTPClient(id, &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				<-req.Context().Done()
				return nil, req.Context().Err()
			}),
		})

		_, _, err := client.ClassifyURL("https://example.com", 10*time.Millisecond)
		if err == nil {
			t.Fatal("expected timeout error, got nil")
		}
		if !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "deadline exceeded") {
			t.Errorf("expected timeout/deadline exceeded error, got: %v", err)
		}
	})

	t.Run("Response Too Large", func(t *testing.T) {
		client := NewClientWithHTTPClient(id, &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Body:       io.NopCloser(strings.NewReader("hello")),
					Header:     make(http.Header),
					Request:    req,
				}, nil
			}),
		})
		client.SetMaxResponseBytes(4)

		_, _, err := client.ClassifyURL("https://example.com", time.Second)
		if err == nil {
			t.Fatal("expected size limit error, got nil")
		}
		if !strings.Contains(err.Error(), "exceeds 4 bytes") {
			t.Errorf("expected size limit error, got %v", err)
		}
	})
}
