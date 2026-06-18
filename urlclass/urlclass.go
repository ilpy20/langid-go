package urlclass

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ilpy20/langid-go"
)

// Client is a URL classification client.
type Client struct {
	id *langid.Identifier
}

// NewClient creates a new Client using the provided identifier.
func NewClient(id *langid.Identifier) *Client {
	return &Client{
		id: id,
	}
}

// ClassifyURL fetches the content of targetURL and classifies its language.
// The request will be timed out if it takes longer than the specified duration.
func (c *Client) ClassifyURL(targetURL string, timeout time.Duration) (langid.Result, int, error) {
	body, err := fetchURL(targetURL, timeout)
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
	body, err := fetchURL(targetURL, timeout)
	if err != nil {
		return nil, 0, err
	}

	res, err := c.id.RankBytes(body)
	if err != nil {
		return nil, 0, err
	}

	return res, len(body), nil
}

func fetchURL(targetURL string, timeout time.Duration) ([]byte, error) {
	client := &http.Client{
		Timeout: timeout,
	}

	resp, err := client.Get(targetURL)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http error: status code %d %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return body, nil
}
