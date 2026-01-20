package handlers

import (
	"net/http"
	"trade-backend-go/internal/clients"

	"github.com/gin-gonic/gin"
)

type StrategyHandler struct {
	client *clients.StrategyClient
}

func NewStrategyHandler(client *clients.StrategyClient) *StrategyHandler {
	return &StrategyHandler{
		client: client,
	}
}

// Helper to proxy client response
func (h *StrategyHandler) proxyResponse(c *gin.Context, data []byte, statusCode int, err error) {
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"success": false,
			"message": "Strategy service unavailable",
			"error":   err.Error(),
		})
		return
	}
	c.Data(statusCode, "application/json", data)
}

// Strategy CRUD

func (h *StrategyHandler) ListStrategies(c *gin.Context) {
	data, code, err := h.client.ListStrategies()
	h.proxyResponse(c, data, code, err)
}

func (h *StrategyHandler) GetStrategy(c *gin.Context) {
	id := c.Param("id")
	data, code, err := h.client.GetStrategy(id)
	h.proxyResponse(c, data, code, err)
}

func (h *StrategyHandler) UploadStrategy(c *gin.Context) {
	// Get file from form
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "File is required",
			"error":   err.Error(),
		})
		return
	}
	defer file.Close()

	name := c.PostForm("name")
	description := c.PostForm("description")

	data, code, err := h.client.UploadStrategy(file, header.Filename, name, description)
	h.proxyResponse(c, data, code, err)
}

func (h *StrategyHandler) UpdateStrategy(c *gin.Context) {
	id := c.Param("id")
	body, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request body",
			"error":   err.Error(),
		})
		return
	}

	data, code, err := h.client.UpdateStrategy(id, body)
	h.proxyResponse(c, data, code, err)
}

func (h *StrategyHandler) DeleteStrategy(c *gin.Context) {
	id := c.Param("id")
	data, code, err := h.client.DeleteStrategy(id)
	h.proxyResponse(c, data, code, err)
}

func (h *StrategyHandler) GetStrategyParameters(c *gin.Context) {
	id := c.Param("id")
	data, code, err := h.client.GetStrategyParameters(id)
	h.proxyResponse(c, data, code, err)
}

func (h *StrategyHandler) ValidateStrategy(c *gin.Context) {
	id := c.Param("id")
	data, code, err := h.client.ValidateStrategy(id)
	h.proxyResponse(c, data, code, err)
}

// Configuration CRUD

func (h *StrategyHandler) GetConfigurations(c *gin.Context) {
	id := c.Param("id")
	data, code, err := h.client.GetConfigurations(id)
	h.proxyResponse(c, data, code, err)
}

func (h *StrategyHandler) GetConfiguration(c *gin.Context) {
	id := c.Param("configId")
	data, code, err := h.client.GetConfiguration(id)
	h.proxyResponse(c, data, code, err)
}

func (h *StrategyHandler) CreateConfiguration(c *gin.Context) {
	id := c.Param("id")
	body, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request body",
			"error":   err.Error(),
		})
		return
	}

	data, code, err := h.client.CreateConfiguration(id, body)
	h.proxyResponse(c, data, code, err)
}

func (h *StrategyHandler) UpdateConfiguration(c *gin.Context) {
	id := c.Param("configId")
	body, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request body",
			"error":   err.Error(),
		})
		return
	}

	data, code, err := h.client.UpdateConfiguration(id, body)
	h.proxyResponse(c, data, code, err)
}

func (h *StrategyHandler) DeleteConfiguration(c *gin.Context) {
	id := c.Param("configId")
	data, code, err := h.client.DeleteConfiguration(id)
	h.proxyResponse(c, data, code, err)
}

// Backtesting

func (h *StrategyHandler) RunBacktest(c *gin.Context) {
	id := c.Param("id")
	body, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request body",
			"error":   err.Error(),
		})
		return
	}

	data, code, err := h.client.RunBacktest(id, body)
	h.proxyResponse(c, data, code, err)
}

func (h *StrategyHandler) GetBacktestRuns(c *gin.Context) {
	var strategyID *string
	if id := c.Query("strategy_id"); id != "" {
		strategyID = &id
	}
	data, code, err := h.client.GetBacktestRuns(strategyID)
	h.proxyResponse(c, data, code, err)
}

func (h *StrategyHandler) GetBacktestRun(c *gin.Context) {
	id := c.Param("runId")
	data, code, err := h.client.GetBacktestRun(id)
	h.proxyResponse(c, data, code, err)
}

func (h *StrategyHandler) DeleteBacktestRun(c *gin.Context) {
	id := c.Param("runId")
	data, code, err := h.client.DeleteBacktestRun(id)
	h.proxyResponse(c, data, code, err)
}

// Live Trading

func (h *StrategyHandler) StartStrategy(c *gin.Context) {
	id := c.Param("id")
	body, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request body",
			"error":   err.Error(),
		})
		return
	}

	data, code, err := h.client.StartStrategy(id, body)
	h.proxyResponse(c, data, code, err)
}

func (h *StrategyHandler) StopStrategy(c *gin.Context) {
	id := c.Param("id")
	data, code, err := h.client.StopStrategy(id)
	h.proxyResponse(c, data, code, err)
}

func (h *StrategyHandler) PauseStrategy(c *gin.Context) {
	id := c.Param("id")
	data, code, err := h.client.PauseStrategy(id)
	h.proxyResponse(c, data, code, err)
}

func (h *StrategyHandler) ResumeStrategy(c *gin.Context) {
	id := c.Param("id")
	data, code, err := h.client.ResumeStrategy(id)
	h.proxyResponse(c, data, code, err)
}

func (h *StrategyHandler) GetStrategyStatus(c *gin.Context) {
	id := c.Param("id")
	data, code, err := h.client.GetStrategyStatus(id)
	h.proxyResponse(c, data, code, err)
}

func (h *StrategyHandler) GetExecutionStats(c *gin.Context) {
	data, code, err := h.client.GetExecutionStats()
	h.proxyResponse(c, data, code, err)
}

// Data Import

func (h *StrategyHandler) ListImportFiles(c *gin.Context) {
	data, code, err := h.client.ListImportFiles()
	h.proxyResponse(c, data, code, err)
}

func (h *StrategyHandler) ListImportFilesDetailed(c *gin.Context) {
	data, code, err := h.client.ListImportFilesDetailed()
	h.proxyResponse(c, data, code, err)
}

func (h *StrategyHandler) GetImportedData(c *gin.Context) {
	expand := c.DefaultQuery("expand", "false") == "true"
	data, code, err := h.client.GetImportedData(expand)
	h.proxyResponse(c, data, code, err)
}

func (h *StrategyHandler) GetImportedDataBySymbol(c *gin.Context) {
	symbol := c.Param("symbol")
	data, code, err := h.client.GetImportedDataBySymbol(symbol)
	h.proxyResponse(c, data, code, err)
}

func (h *StrategyHandler) GetFileMetadata(c *gin.Context) {
	filename := c.Param("filename")
	data, code, err := h.client.GetFileMetadata(filename)
	h.proxyResponse(c, data, code, err)
}

func (h *StrategyHandler) CreateImportJob(c *gin.Context) {
	body, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request body",
			"error":   err.Error(),
		})
		return
	}
	data, code, err := h.client.CreateImportJob(body)
	h.proxyResponse(c, data, code, err)
}

func (h *StrategyHandler) GetImportJobStatus(c *gin.Context) {
	jobID := c.Param("job_id")
	data, code, err := h.client.GetImportJobStatus(jobID)
	h.proxyResponse(c, data, code, err)
}

func (h *StrategyHandler) ListImportJobs(c *gin.Context) {
	status := c.Query("status")
	limit := 50
	data, code, err := h.client.ListImportJobs(status, limit)
	h.proxyResponse(c, data, code, err)
}

func (h *StrategyHandler) CancelImportJob(c *gin.Context) {
	jobID := c.Param("job_id")
	data, code, err := h.client.CancelImportJob(jobID)
	h.proxyResponse(c, data, code, err)
}

func (h *StrategyHandler) GetImportQueueStatus(c *gin.Context) {
	data, code, err := h.client.GetImportQueueStatus()
	h.proxyResponse(c, data, code, err)
}

func (h *StrategyHandler) AddToImportQueue(c *gin.Context) {
	body, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request body",
			"error":   err.Error(),
		})
		return
	}
	data, code, err := h.client.AddToImportQueue(body)
	h.proxyResponse(c, data, code, err)
}

func (h *StrategyHandler) AddBatchToImportQueue(c *gin.Context) {
	body, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request body",
			"error":   err.Error(),
		})
		return
	}
	data, code, err := h.client.AddBatchToImportQueue(body)
	h.proxyResponse(c, data, code, err)
}

func (h *StrategyHandler) RemoveFromImportQueue(c *gin.Context) {
	queueID := c.Param("queue_id")
	data, code, err := h.client.RemoveFromImportQueue(queueID)
	h.proxyResponse(c, data, code, err)
}

func (h *StrategyHandler) ClearImportQueue(c *gin.Context) {
	data, code, err := h.client.ClearImportQueue()
	h.proxyResponse(c, data, code, err)
}

func (h *StrategyHandler) GetImportSummary(c *gin.Context) {
	data, code, err := h.client.GetImportSummary()
	h.proxyResponse(c, data, code, err)
}

func (h *StrategyHandler) GetStorageStats(c *gin.Context) {
	data, code, err := h.client.GetStorageStats()
	h.proxyResponse(c, data, code, err)
}
