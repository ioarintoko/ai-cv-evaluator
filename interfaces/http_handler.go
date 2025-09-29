package interfaces

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"cv-evaluator/domain"
	"cv-evaluator/infrastructure"
)

type HTTPHandler struct {
	DB  *gorm.DB
	RMQ *infrastructure.RabbitMQ
}

func NewHTTPHandler(router *gin.Engine, db *gorm.DB, rmq *infrastructure.RabbitMQ) {
	h := &HTTPHandler{DB: db, RMQ: rmq}

	router.POST("/upload", h.UploadMultipleFiles)
	router.POST("/evaluate", h.Evaluate)
	router.GET("/result/:id", h.GetResult)
}

// UploadMultipleFiles menerima CV + Project, ekstrak teks, simpan ke DB
func (h *HTTPHandler) UploadMultipleFiles(c *gin.Context) {
	candidateName := c.PostForm("candidate_name")
	candidateEmail := c.PostForm("candidate_email")

	cvHeader, err := c.FormFile("cv_file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cv_file is required"})
		return
	}
	cvFile, err := cvHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to open CV file"})
		return
	}
	defer cvFile.Close()

	projectHeader, err := c.FormFile("project_file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "project_file is required"})
		return
	}
	projectFile, err := projectHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to open Project file"})
		return
	}
	defer projectFile.Close()

	aiClient := infrastructure.NewGeminiClient()

	// FIX: Pass both file and filename to ExtractTextFromFile
	cvText, err := aiClient.ExtractTextFromFile(cvFile, cvHeader.Filename) // Added filename
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to extract CV text: " + err.Error()})
		return
	}

	projectText, err := aiClient.ExtractTextFromFile(projectFile, projectHeader.Filename) // Added filename
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to extract Project text: " + err.Error()})
		return
	}

	upload := domain.Upload{
		CandidateName:  candidateName,
		CandidateEmail: candidateEmail,
		CVText:         cvText,
		ProjectText:    projectText,
	}

	if err := h.DB.Create(&upload).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save upload: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"upload_id": upload.ID,
		"message":   "Files uploaded and processed successfully",
	})
}

// Evaluate → panggil Gemini untuk evaluasi
func (h *HTTPHandler) Evaluate(c *gin.Context) {
	var req struct {
		UploadID uint `json:"upload_id"`
		JobID    uint `json:"job_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var upload domain.Upload
	if err := h.DB.First(&upload, req.UploadID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "upload not found"})
		return
	}

	var job domain.Job
	if err := h.DB.First(&job, req.JobID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}

	// Create evaluation record dengan status "queued"
	eval := domain.Evaluation{
		UploadID: upload.ID,
		JobID:    job.ID,
		Status:   "queued",
	}

	if err := h.DB.Create(&eval).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create evaluation"})
		return
	}

	// Queue job ke RabbitMQ (async)
	jobData := infrastructure.EvaluationJob{
		EvaluationID: eval.ID,
		UploadID:     req.UploadID,
		JobID:        req.JobID,
	}
	if err := h.RMQ.PublishJob(jobData); err != nil {
		// Update status ke failed jika queue gagal
		h.DB.Model(&domain.Evaluation{}).
			Where("id = ?", eval.ID).
			Update("status", "failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to queue job"})
		return
	}

	// RETURN IMMEDIATELY dengan status queued
	c.JSON(http.StatusOK, gin.H{
		"id":     eval.ID,
		"status": "queued",
	})
}

// GetResult ambil hasil evaluasi
func (h *HTTPHandler) GetResult(c *gin.Context) {
	idStr := strings.TrimSpace(c.Param("id"))
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var eval domain.Evaluation
	if err := h.DB.First(&eval, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "evaluation not found"})
		return
	}

	resp := gin.H{
		"id":         eval.ID,
		"status":     eval.Status,
		"upload_id":  eval.UploadID,
		"job_id":     eval.JobID,
		"created_at": eval.CreatedAt,
		"updated_at": eval.UpdatedAt,
	}

	if eval.Status == "completed" {
		resp["result"] = gin.H{
			"cv_match_rate":    eval.CVMatchRate,
			"cv_feedback":      eval.CVFeedback,
			"project_score":    eval.ProjectScore,
			"project_feedback": eval.ProjectFeedback,
			"overall_summary":  eval.OverallSummary,
		}
	}

	c.JSON(http.StatusOK, resp)
}
