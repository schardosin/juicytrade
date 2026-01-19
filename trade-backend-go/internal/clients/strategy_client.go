package clients

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

type StrategyClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewStrategyClient(baseURL string) *StrategyClient {
	return &StrategyClient{
		baseURL:    baseURL,
		httpClient: &http.Client{}, // No default timeout, controlled by context
	}
}

// Helper to make requests and return raw response body
func (c *StrategyClient) doRequest(ctx context.Context, method, path string, body io.Reader, contentType string) ([]byte, int, error) {
	// Apply default timeout if not set
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
	}

	url := fmt.Sprintf("%s%s", c.baseURL, path)
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}

	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response body: %w", err)
	}

	return respBody, resp.StatusCode, nil
}

// Strategy CRUD

func (c *StrategyClient) ListStrategies() ([]byte, int, error) {
	return c.doRequest(context.Background(), "GET", "/api/strategies/my", nil, "")
}

func (c *StrategyClient) GetStrategy(strategyID string) ([]byte, int, error) {
	return c.doRequest(context.Background(), "GET", fmt.Sprintf("/api/strategies/%s", strategyID), nil, "")
}

func (c *StrategyClient) UploadStrategy(file io.Reader, filename, name, description string) ([]byte, int, error) {
	// Create multipart body
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add file
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, 0, fmt.Errorf("failed to copy file content: %w", err)
	}

	// Add fields
	if err := writer.WriteField("name", name); err != nil {
		return nil, 0, fmt.Errorf("failed to write name field: %w", err)
	}
	if err := writer.WriteField("description", description); err != nil {
		return nil, 0, fmt.Errorf("failed to write description field: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, 0, fmt.Errorf("failed to close writer: %w", err)
	}

	// Use longer timeout for upload
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	return c.doRequest(ctx, "POST", "/api/strategies/upload", body, writer.FormDataContentType())
}

func (c *StrategyClient) UpdateStrategy(strategyID string, body []byte) ([]byte, int, error) {
	return c.doRequest(context.Background(), "PUT", fmt.Sprintf("/api/strategies/%s", strategyID), bytes.NewReader(body), "application/json")
}

func (c *StrategyClient) DeleteStrategy(strategyID string) ([]byte, int, error) {
	return c.doRequest(context.Background(), "DELETE", fmt.Sprintf("/api/strategies/%s", strategyID), nil, "")
}

func (c *StrategyClient) GetStrategyParameters(strategyID string) ([]byte, int, error) {
	return c.doRequest(context.Background(), "GET", fmt.Sprintf("/api/strategies/%s/parameters", strategyID), nil, "")
}

func (c *StrategyClient) ValidateStrategy(strategyID string) ([]byte, int, error) {
	return c.doRequest(context.Background(), "POST", fmt.Sprintf("/api/strategies/%s/validate", strategyID), nil, "")
}

// Configuration CRUD

func (c *StrategyClient) GetConfigurations(strategyID string) ([]byte, int, error) {
	return c.doRequest(context.Background(), "GET", fmt.Sprintf("/api/strategies/%s/configs", strategyID), nil, "")
}

func (c *StrategyClient) GetConfiguration(configID string) ([]byte, int, error) {
	return c.doRequest(context.Background(), "GET", fmt.Sprintf("/api/strategies/configs/%s", configID), nil, "")
}

func (c *StrategyClient) CreateConfiguration(strategyID string, body []byte) ([]byte, int, error) {
	return c.doRequest(context.Background(), "POST", fmt.Sprintf("/api/strategies/%s/configs", strategyID), bytes.NewReader(body), "application/json")
}

func (c *StrategyClient) UpdateConfiguration(configID string, body []byte) ([]byte, int, error) {
	return c.doRequest(context.Background(), "PUT", fmt.Sprintf("/api/strategies/configs/%s", configID), bytes.NewReader(body), "application/json")
}

func (c *StrategyClient) DeleteConfiguration(configID string) ([]byte, int, error) {
	return c.doRequest(context.Background(), "DELETE", fmt.Sprintf("/api/strategies/configs/%s", configID), nil, "")
}

// Backtesting

func (c *StrategyClient) RunBacktest(strategyID string, body []byte) ([]byte, int, error) {
	// 30 minute timeout for backtests (2+ months of 1min data needs significant time)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	return c.doRequest(ctx, "POST", fmt.Sprintf("/api/strategies/%s/backtest", strategyID), bytes.NewReader(body), "application/json")
}

func (c *StrategyClient) GetBacktestRuns(strategyID *string) ([]byte, int, error) {
	path := "/api/strategies/backtest/runs"
	if strategyID != nil {
		path = fmt.Sprintf("%s?strategy_id=%s", path, *strategyID)
	}
	return c.doRequest(context.Background(), "GET", path, nil, "")
}

func (c *StrategyClient) GetBacktestRun(runID string) ([]byte, int, error) {
	return c.doRequest(context.Background(), "GET", fmt.Sprintf("/api/strategies/backtest/runs/%s", runID), nil, "")
}

func (c *StrategyClient) DeleteBacktestRun(runID string) ([]byte, int, error) {
	return c.doRequest(context.Background(), "DELETE", fmt.Sprintf("/api/strategies/backtest/runs/%s", runID), nil, "")
}

// Live Trading

func (c *StrategyClient) StartStrategy(strategyID string, body []byte) ([]byte, int, error) {
	return c.doRequest(context.Background(), "POST", fmt.Sprintf("/api/strategies/%s/start", strategyID), bytes.NewReader(body), "application/json")
}

func (c *StrategyClient) StopStrategy(strategyID string) ([]byte, int, error) {
	return c.doRequest(context.Background(), "POST", fmt.Sprintf("/api/strategies/%s/stop", strategyID), nil, "")
}

func (c *StrategyClient) PauseStrategy(strategyID string) ([]byte, int, error) {
	return c.doRequest(context.Background(), "POST", fmt.Sprintf("/api/strategies/%s/pause", strategyID), nil, "")
}

func (c *StrategyClient) ResumeStrategy(strategyID string) ([]byte, int, error) {
	return c.doRequest(context.Background(), "POST", fmt.Sprintf("/api/strategies/%s/resume", strategyID), nil, "")
}

func (c *StrategyClient) GetStrategyStatus(strategyID string) ([]byte, int, error) {
	return c.doRequest(context.Background(), "GET", fmt.Sprintf("/api/strategies/%s/status", strategyID), nil, "")
}

func (c *StrategyClient) GetExecutionStats() ([]byte, int, error) {
	return c.doRequest(context.Background(), "GET", "/api/strategies/stats", nil, "")
}
