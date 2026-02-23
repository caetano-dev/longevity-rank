package scraper

import (
	"io"
	"net/http"
	"time"
)

const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

// DefaultClient is a shared HTTP client used by all scrapers.
var DefaultClient = &http.Client{Timeout: 30 * time.Second}

// NewRequest creates a GET request with the standard User-Agent header.
func NewRequest(url string) (*http.Request, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	return req, nil
}

// FetchBody performs a GET request and returns the response body bytes.
func FetchBody(url string) ([]byte, error) {
	req, err := NewRequest(url)
	if err != nil {
		return nil, err
	}
	resp, err := DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}