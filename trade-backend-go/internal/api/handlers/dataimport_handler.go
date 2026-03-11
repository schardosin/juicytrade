package handlers

import (
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"trade-backend-go/internal/clients"
)

// DataImportHandler handles data import API requests by proxying to Python backend
type DataImportHandler struct {
	client *clients.DataImportClient
}

func NewDataImportHandler(client *clients.DataImportClient) *DataImportHandler {
	return &DataImportHandler{client: client}
}

// Helper to write response from client
func (h *DataImportHandler) writeResponse(c *gin.Context, body []byte, statusCode int, err error) {
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Data import service unavailable", "details": err.Error()})
		return
	}
	c.Data(statusCode, "application/json", body)
}

// Files

func (h *DataImportHandler) ListFiles(c *gin.Context) {
	body, status, err := h.client.ListFiles()
	h.writeResponse(c, body, status, err)
}

func (h *DataImportHandler) ListFilesDetailed(c *gin.Context) {
	body, status, err := h.client.ListFilesDetailed()
	h.writeResponse(c, body, status, err)
}

// Imported data

func (h *DataImportHandler) GetImportedData(c *gin.Context) {
	expandStr := c.DefaultQuery("expand", "false")
	expand := expandStr == "true"
	body, status, err := h.client.GetImportedData(expand)
	h.writeResponse(c, body, status, err)
}

func (h *DataImportHandler) GetImportedDataBySymbol(c *gin.Context) {
	symbol := c.Param("symbol")
	assetType := c.DefaultQuery("asset_type", "")
	body, status, err := h.client.GetImportedDataBySymbol(symbol, assetType)
	h.writeResponse(c, body, status, err)
}

// Metadata

func (h *DataImportHandler) GetFileMetadata(c *gin.Context) {
	filename := c.Param("filename")
	symbol := c.Query("symbol")
	forceRefreshStr := c.DefaultQuery("force_refresh", "false")
	forceRefresh := forceRefreshStr == "true"
	body, status, err := h.client.GetFileMetadata(filename, symbol, forceRefresh)
	h.writeResponse(c, body, status, err)
}

// Jobs

func (h *DataImportHandler) CreateJob(c *gin.Context) {
	reqBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}
	body, status, clientErr := h.client.CreateJob(reqBody)
	h.writeResponse(c, body, status, clientErr)
}

func (h *DataImportHandler) GetJobStatus(c *gin.Context) {
	jobID := c.Param("job_id")
	body, status, err := h.client.GetJobStatus(jobID)
	h.writeResponse(c, body, status, err)
}

func (h *DataImportHandler) ListJobs(c *gin.Context) {
	status := c.Query("status")
	limitStr := c.DefaultQuery("limit", "50")
	limit, _ := strconv.Atoi(limitStr)
	body, statusCode, err := h.client.ListJobs(status, limit)
	h.writeResponse(c, body, statusCode, err)
}

func (h *DataImportHandler) CancelJob(c *gin.Context) {
	jobID := c.Param("job_id")
	body, status, err := h.client.CancelJob(jobID)
	h.writeResponse(c, body, status, err)
}

// Queue

func (h *DataImportHandler) GetQueueStatus(c *gin.Context) {
	body, status, err := h.client.GetQueueStatus()
	h.writeResponse(c, body, status, err)
}

func (h *DataImportHandler) AddToQueue(c *gin.Context) {
	reqBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}
	body, status, clientErr := h.client.AddToQueue(reqBody)
	h.writeResponse(c, body, status, clientErr)
}

func (h *DataImportHandler) AddBatchToQueue(c *gin.Context) {
	reqBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}
	body, status, clientErr := h.client.AddBatchToQueue(reqBody)
	h.writeResponse(c, body, status, clientErr)
}

func (h *DataImportHandler) RemoveFromQueue(c *gin.Context) {
	queueID := c.Param("queue_id")
	body, status, err := h.client.RemoveFromQueue(queueID)
	h.writeResponse(c, body, status, err)
}

func (h *DataImportHandler) ClearQueue(c *gin.Context) {
	body, status, err := h.client.ClearQueue()
	h.writeResponse(c, body, status, err)
}

// Summary and stats

func (h *DataImportHandler) GetImportSummary(c *gin.Context) {
	body, status, err := h.client.GetImportSummary()
	h.writeResponse(c, body, status, err)
}

func (h *DataImportHandler) GetStorageStats(c *gin.Context) {
	body, status, err := h.client.GetStorageStats()
	h.writeResponse(c, body, status, err)
}
