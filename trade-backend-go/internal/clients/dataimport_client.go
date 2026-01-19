package clients

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DataImportClient handles communication with the Python backend for data import operations
type DataImportClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewDataImportClient(baseURL string) *DataImportClient {
	return &DataImportClient{
		baseURL:    baseURL,
		httpClient: &http.Client{},
	}
}

// doRequest makes an HTTP request and returns the raw response body
func (c *DataImportClient) doRequest(ctx context.Context, method, path string, body io.Reader, contentType string) ([]byte, int, error) {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
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

// File listing

func (c *DataImportClient) ListFiles() ([]byte, int, error) {
	return c.doRequest(context.Background(), "GET", "/api/data-import/files", nil, "")
}

func (c *DataImportClient) ListFilesDetailed() ([]byte, int, error) {
	return c.doRequest(context.Background(), "GET", "/api/data-import/files/detailed", nil, "")
}

// Imported data

func (c *DataImportClient) GetImportedData(expand bool) ([]byte, int, error) {
	expandStr := "false"
	if expand {
		expandStr = "true"
	}
	return c.doRequest(context.Background(), "GET", fmt.Sprintf("/api/data-import/imported-data?expand=%s", expandStr), nil, "")
}

func (c *DataImportClient) GetImportedDataBySymbol(symbol, assetType string) ([]byte, int, error) {
	return c.doRequest(context.Background(), "GET", fmt.Sprintf("/api/data-import/imported-data/%s?asset_type=%s", symbol, assetType), nil, "")
}

// Metadata

func (c *DataImportClient) GetFileMetadata(filename string, symbol string, forceRefresh bool) ([]byte, int, error) {
	path := fmt.Sprintf("/api/data-import/metadata/%s?force_refresh=%t", filename, forceRefresh)
	if symbol != "" {
		path = fmt.Sprintf("/api/data-import/metadata/%s?symbol=%s&force_refresh=%t", filename, symbol, forceRefresh)
	}
	return c.doRequest(context.Background(), "GET", path, nil, "")
}

// Jobs

func (c *DataImportClient) CreateJob(body []byte) ([]byte, int, error) {
	return c.doRequest(context.Background(), "POST", "/api/data-import/jobs", bytes.NewReader(body), "application/json")
}

func (c *DataImportClient) GetJobStatus(jobID string) ([]byte, int, error) {
	return c.doRequest(context.Background(), "GET", fmt.Sprintf("/api/data-import/jobs/%s/status", jobID), nil, "")
}

func (c *DataImportClient) ListJobs(status string, limit int) ([]byte, int, error) {
	path := fmt.Sprintf("/api/data-import/jobs?limit=%d", limit)
	if status != "" {
		path = fmt.Sprintf("/api/data-import/jobs?status=%s&limit=%d", status, limit)
	}
	return c.doRequest(context.Background(), "GET", path, nil, "")
}

func (c *DataImportClient) CancelJob(jobID string) ([]byte, int, error) {
	return c.doRequest(context.Background(), "DELETE", fmt.Sprintf("/api/data-import/jobs/%s", jobID), nil, "")
}

// Queue

func (c *DataImportClient) GetQueueStatus() ([]byte, int, error) {
	return c.doRequest(context.Background(), "GET", "/api/data-import/queue/status", nil, "")
}

func (c *DataImportClient) AddToQueue(body []byte) ([]byte, int, error) {
	return c.doRequest(context.Background(), "POST", "/api/data-import/queue", bytes.NewReader(body), "application/json")
}

func (c *DataImportClient) AddBatchToQueue(body []byte) ([]byte, int, error) {
	return c.doRequest(context.Background(), "POST", "/api/data-import/queue/batch", bytes.NewReader(body), "application/json")
}

func (c *DataImportClient) RemoveFromQueue(queueID string) ([]byte, int, error) {
	return c.doRequest(context.Background(), "DELETE", fmt.Sprintf("/api/data-import/queue/%s", queueID), nil, "")
}

func (c *DataImportClient) ClearQueue() ([]byte, int, error) {
	return c.doRequest(context.Background(), "DELETE", "/api/data-import/queue/clear", nil, "")
}

// Summary and stats

func (c *DataImportClient) GetImportSummary() ([]byte, int, error) {
	return c.doRequest(context.Background(), "GET", "/data/import/summary", nil, "")
}

func (c *DataImportClient) GetStorageStats() ([]byte, int, error) {
	return c.doRequest(context.Background(), "GET", "/data/storage/stats", nil, "")
}
