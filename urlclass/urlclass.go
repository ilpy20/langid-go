package urlclass

import (
	"errors"
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
	// Skeleton implementation to be fully completed in Phase 2
	return langid.Result{}, 0, errors.New("url classification not implemented yet")
}

// RankURL fetches the content of targetURL and ranks all supported languages.
// The request will be timed out if it takes longer than the specified duration.
func (c *Client) RankURL(targetURL string, timeout time.Duration) ([]langid.Result, int, error) {
	// Skeleton implementation to be fully completed in Phase 2
	return nil, 0, errors.New("url ranking not implemented yet")
}
