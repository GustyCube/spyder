package httpclient

import (
	"context"
	"crypto/tls"
	"net/http"
	"time"

	"github.com/gustycube/spyder/internal/circuitbreaker"
)

// ClientConfig holds HTTP client transport and timeout settings.
type ClientConfig struct {
	Timeout               time.Duration
	ResponseHeaderTimeout time.Duration
	IdleConnTimeout       time.Duration
	MaxIdleConns          int
	MaxConnsPerHost       int
}

// ResilientClient wraps http.Client with circuit breaker functionality
type ResilientClient struct {
	client      *http.Client
	hostBreaker *circuitbreaker.HostBreaker
}

// NewResilientClient creates a new HTTP client with circuit breaker.
// It builds the underlying *http.Client and *http.Transport from cfg,
// and creates a per-host circuit breaker from cbCfg.
func NewResilientClient(cfg ClientConfig, cbCfg circuitbreaker.Config) *ResilientClient {
	tr := &http.Transport{
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: false},
		DisableCompression:    false,
		MaxIdleConns:          cfg.MaxIdleConns,
		MaxConnsPerHost:       cfg.MaxConnsPerHost,
		MaxIdleConnsPerHost:   cfg.MaxIdleConns / 16, // reasonable default
		ResponseHeaderTimeout: cfg.ResponseHeaderTimeout,
		IdleConnTimeout:       cfg.IdleConnTimeout,
		ExpectContinueTimeout: 1 * time.Second,
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   cfg.Timeout,
	}

	return &ResilientClient{
		client:      client,
		hostBreaker: circuitbreaker.NewHostBreaker(&cbCfg),
	}
}

// Do executes an HTTP request with circuit breaker protection
func (c *ResilientClient) Do(req *http.Request) (*http.Response, error) {
	host := req.URL.Host
	if host == "" {
		host = req.URL.Hostname()
	}

	var resp *http.Response
	err := c.hostBreaker.Execute(host, func() error {
		var err error
		resp, err = c.client.Do(req)

		// Consider 5xx errors and connection errors as failures
		if err != nil {
			return err
		}

		if resp.StatusCode >= 500 {
			return &HTTPError{
				StatusCode: resp.StatusCode,
				Status:     resp.Status,
			}
		}

		return nil
	})

	return resp, err
}

// Get performs a GET request with circuit breaker
func (c *ResilientClient) Get(url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(req)
}

// GetWithContext performs a GET request with context and circuit breaker
func (c *ResilientClient) GetWithContext(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(req)
}

// Stats returns circuit breaker statistics for all hosts
func (c *ResilientClient) Stats() map[string]struct {
	State    string
	Requests uint32
	Failures uint32
} {
	return c.hostBreaker.Stats()
}

// ResetBreaker resets the circuit breaker for a specific host
func (c *ResilientClient) ResetBreaker(host string) {
	c.hostBreaker.Reset(host)
}

// HTTPError represents an HTTP error response
type HTTPError struct {
	StatusCode int
	Status     string
}

func (e *HTTPError) Error() string {
	return e.Status
}

// IsHTTPError checks if an error is an HTTPError
func IsHTTPError(err error) bool {
	_, ok := err.(*HTTPError)
	return ok
}

// GetHTTPStatusCode returns the HTTP status code from an HTTPError
func GetHTTPStatusCode(err error) int {
	if httpErr, ok := err.(*HTTPError); ok {
		return httpErr.StatusCode
	}
	return 0
}
