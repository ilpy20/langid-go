package urlclass

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ilpy20/langid-go"
)

const DefaultMaxResponseBytes int64 = 4 << 20

// Client is a URL classification client.
type Client struct {
	id               *langid.Identifier
	httpClient       *http.Client
	maxResponseBytes int64
}

// NewClient creates a new Client using the provided identifier.
func NewClient(id *langid.Identifier) *Client {
	return &Client{
		id:               id,
		httpClient:       http.DefaultClient,
		maxResponseBytes: DefaultMaxResponseBytes,
	}
}

// NewClientWithHTTPClient creates a new Client using the provided identifier and HTTP client.
func NewClientWithHTTPClient(id *langid.Identifier, httpClient *http.Client) *Client {
	c := NewClient(id)
	if httpClient != nil {
		c.httpClient = httpClient
	}
	return c
}

// SetMaxResponseBytes sets the maximum response body size accepted from fetched URLs.
// Non-positive values disable the limit.
func (c *Client) SetMaxResponseBytes(n int64) {
	if c == nil {
		return
	}
	c.maxResponseBytes = n
}

// ClassifyURL fetches the content of targetURL and classifies its language.
// The request will be timed out if it takes longer than the specified duration.
func (c *Client) ClassifyURL(targetURL string, timeout time.Duration) (langid.Result, int, error) {
	body, err := c.fetchURL(targetURL, timeout)
	if err != nil {
		return langid.Result{}, 0, err
	}

	res, err := c.id.IdentifyBytes(body)
	if err != nil {
		return langid.Result{}, 0, err
	}

	return res, len(body), nil
}

// RankURL fetches the content of targetURL and ranks all supported languages.
// The request will be timed out if it takes longer than the specified duration.
func (c *Client) RankURL(targetURL string, timeout time.Duration) ([]langid.Result, int, error) {
	body, err := c.fetchURL(targetURL, timeout)
	if err != nil {
		return nil, 0, err
	}

	res, err := c.id.RankBytes(body)
	if err != nil {
		return nil, 0, err
	}

	return res, len(body), nil
}

func (c *Client) fetchURL(targetURL string, timeout time.Duration) ([]byte, error) {
	baseClient := c.httpClient
	if baseClient == nil {
		baseClient = http.DefaultClient
	}
	client := *baseClient
	client.Timeout = timeout

	resp, err := client.Get(targetURL)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http error: status code %d %s", resp.StatusCode, resp.Status)
	}

	bodyReader := io.Reader(resp.Body)
	if c.maxResponseBytes > 0 {
		bodyReader = io.LimitReader(resp.Body, c.maxResponseBytes+1)
	}
	body, err := io.ReadAll(bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	if c.maxResponseBytes > 0 && int64(len(body)) > c.maxResponseBytes {
		return nil, fmt.Errorf("response body exceeds %d bytes", c.maxResponseBytes)
	}

	return body, nil
}
