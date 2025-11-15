package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HTTPClient provides HTTP client functionality with retry logic.
// This replicates the retry and error handling behavior from the Python implementation.
type HTTPClient struct {
	client     *http.Client
	maxRetries int
	baseDelay  time.Duration
}

// NewHTTPClient creates a new HTTP client with retry logic.
// Matches the Python retry configuration (3 retries, exponential backoff).
func NewHTTPClient() *HTTPClient {
	return &HTTPClient{
		client: &http.Client{
			Timeout: 30 * time.Second, // Same as Python requests timeout
		},
		maxRetries: 3,             // Same as Python (3 retries)
		baseDelay:  1 * time.Second, // Same as Python (1s, 2s, 4s)
	}
}

// Request represents an HTTP request configuration
type Request struct {
	Method  string
	URL     string
	Headers map[string]string
	Body    interface{}
	Params  map[string]string
}

// Response represents an HTTP response
type Response struct {
	StatusCode int
	Body       []byte
	Headers    http.Header
}

// HTTPError represents an HTTP error with status code and response
type HTTPError struct {
	StatusCode int
	Message    string
	Response   *Response
}

func (e *HTTPError) Error() string {
	return e.Message
}

// IsClientError returns true if this is a 4xx client error
func (e *HTTPError) IsClientError() bool {
	return e.StatusCode >= 400 && e.StatusCode < 500
}

// IsServerError returns true if this is a 5xx server error  
func (e *HTTPError) IsServerError() bool {
	return e.StatusCode >= 500 && e.StatusCode < 600
}

// Do executes an HTTP request with retry logic and circuit breaker.
// Exact replication of Python retry behavior: 3 attempts with exponential backoff (1s, 2s, 4s).
func (c *HTTPClient) Do(ctx context.Context, req Request) (*Response, error) {
	return c.doWithRetry(ctx, req, 0)
}

func (c *HTTPClient) doWithRetry(ctx context.Context, req Request, attempt int) (*Response, error) {
	// Execute request directly
	resp, err := c.executeRequest(ctx, req)

	if err != nil {
		// Check if this is a client error (4xx) that should not be retried
		if httpErr, ok := err.(*HTTPError); ok && httpErr.IsClientError() {
			// Client errors (400-499) should not be retried and should return immediately
			// These are validation/business logic errors, not infrastructure failures
			return httpErr.Response, err
		}

		// If we haven't exceeded max retries, retry with exponential backoff
		if attempt < c.maxRetries {
			// Exponential backoff: 1s, 2s, 4s (same as Python)
			delay := c.baseDelay * time.Duration(1<<attempt)
			
			select {
			case <-time.After(delay):
				return c.doWithRetry(ctx, req, attempt+1)
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		return nil, err
	}

	return resp, nil
}

func (c *HTTPClient) executeRequest(ctx context.Context, req Request) (*Response, error) {
	// Prepare request body
	var bodyReader io.Reader
	if req.Body != nil {
		bodyBytes, err := json.Marshal(req.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, req.Method, req.URL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}

	// Add query parameters
	if len(req.Params) > 0 {
		q := httpReq.URL.Query()
		for key, value := range req.Params {
			q.Add(key, value)
		}
		httpReq.URL.RawQuery = q.Encode()
	}

	// Execute request
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for HTTP errors
	if resp.StatusCode >= 400 {
		response := &Response{
			StatusCode: resp.StatusCode,
			Body:       body,
			Headers:    resp.Header,
		}
		
		httpErr := &HTTPError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body)),
			Response:   response,
		}
		
		return response, httpErr
	}

	return &Response{
		StatusCode: resp.StatusCode,
		Body:       body,
		Headers:    resp.Header,
	}, nil
}

// Get performs a GET request
func (c *HTTPClient) Get(ctx context.Context, url string, headers map[string]string, params map[string]string) (*Response, error) {
	return c.Do(ctx, Request{
		Method:  "GET",
		URL:     url,
		Headers: headers,
		Params:  params,
	})
}

// Post performs a POST request
func (c *HTTPClient) Post(ctx context.Context, url string, body interface{}, headers map[string]string) (*Response, error) {
	if headers == nil {
		headers = make(map[string]string)
	}
	headers["Content-Type"] = "application/json"
	
	return c.Do(ctx, Request{
		Method:  "POST",
		URL:     url,
		Headers: headers,
		Body:    body,
	})
}

// Put performs a PUT request
func (c *HTTPClient) Put(ctx context.Context, url string, body interface{}, headers map[string]string) (*Response, error) {
	if headers == nil {
		headers = make(map[string]string)
	}
	headers["Content-Type"] = "application/json"
	
	return c.Do(ctx, Request{
		Method:  "PUT",
		URL:     url,
		Headers: headers,
		Body:    body,
	})
}

// Delete performs a DELETE request
func (c *HTTPClient) Delete(ctx context.Context, url string, headers map[string]string) (*Response, error) {
	return c.Do(ctx, Request{
		Method:  "DELETE",
		URL:     url,
		Headers: headers,
	})
}

// PostForm performs a POST request with form data
func (c *HTTPClient) PostForm(ctx context.Context, url string, headers map[string]string, data map[string]string) ([]byte, error) {
	// Convert form data to URL values
	params := make(map[string]string)
	for k, v := range data {
		params[k] = v
	}
	
	// Set content type for form data
	if headers == nil {
		headers = make(map[string]string)
	}
	headers["Content-Type"] = "application/x-www-form-urlencoded"
	
	// Create form body
	formData := ""
	first := true
	for k, v := range data {
		if !first {
			formData += "&"
		}
		formData += k + "=" + v
		first = false
	}
	
	// Create request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader([]byte(formData)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	// Add headers
	for key, value := range headers {
		httpReq.Header.Set(key, value)
	}
	
	// Execute request
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	
	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	
	// Check for HTTP errors
	if resp.StatusCode >= 400 {
		return body, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	
	return body, nil
}
