package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"cv-evaluator/domain"
	"cv-evaluator/infrastructure"
	"cv-evaluator/interfaces"
)

func main() {
	// Load .env
	_ = godotenv.Load()

	// Connect DB
	db := infrastructure.NewMySQLConnection()

	// Connect RabbitMQ
	rmq := infrastructure.NewRabbitMQ()

	// Init Gemini client
	gemini := infrastructure.NewGeminiClient()

	// Worker consumer → pakai Gemini evaluator
	rmq.ConsumeJobs(func(job infrastructure.EvaluationJob) {
		log.Printf("📥 Worker processing job: %+v\n", job)

		// Update status → processing
		db.Model(&domain.Evaluation{}).
			Where("id = ?", job.EvaluationID).
			Update("status", "processing")

		// Ambil job desc + rubric dari tabel jobs
		var jobMeta domain.Job
		if err := db.First(&jobMeta, job.JobID).Error; err != nil {
			log.Printf("❌ Failed to load job %d: %v", job.JobID, err)
			db.Model(&domain.Evaluation{}).
				Where("id = ?", job.EvaluationID).
				Update("status", "failed")
			return
		}

		// ✅ BENAR: Ambil data dari tabel uploads berdasarkan upload_id
		var upload domain.Upload
		if err := db.First(&upload, job.UploadID).Error; err != nil {
			log.Printf("❌ Failed to load upload %d: %v", job.UploadID, err)
			db.Model(&domain.Evaluation{}).
				Where("id = ?", job.EvaluationID).
				Update("status", "failed")
			return
		}

		// ✅ DETAILED DEBUG LOGGING
		log.Printf("=== 🐛 DEBUG DATA ===")
		log.Printf("📋 Job ID: %d", jobMeta.ID)
		log.Printf("📝 Job Description: %s", jobMeta.Description)
		log.Printf("📊 Job Rubric: %s", jobMeta.Rubric)
		log.Printf("---")
		log.Printf("👤 Upload ID: %d", upload.ID)
		log.Printf("📄 CV Text Length: %d characters", len(upload.CVText))
		log.Printf("📄 CV Text Preview: %.200s", upload.CVText)
		log.Printf("---")
		log.Printf("🚀 Project Text Length: %d characters", len(upload.ProjectText))
		log.Printf("🚀 Project Text Preview: %.200s", upload.ProjectText)
		log.Printf("======================")

		// Panggil Gemini dengan data yang benar dari database
		result, err := gemini.Evaluate(context.Background(),
			jobMeta.Description,
			jobMeta.Rubric,
			upload.CVText,      // ✅ Data dari database
			upload.ProjectText, // ✅ Data dari database
		)
		if err != nil {
			log.Printf("❌ Gemini evaluation error (job %d): %v", job.EvaluationID, err)
			db.Model(&domain.Evaluation{}).
				Where("id = ?", job.EvaluationID).
				Update("status", "failed")
			return
		}

		// Log hasil mentah
		log.Printf("🔎 Gemini raw result for job %d: %+v", job.EvaluationID, result)

		// Simpan hasil
		resultBytes, _ := json.Marshal(result)
		resultStr := string(resultBytes)

		// Cek key cv & project
		cv, ok1 := result["cv"].(map[string]interface{})
		project, ok2 := result["project"].(map[string]interface{})
		if !ok1 || !ok2 {
			log.Printf("❌ Invalid result format for job %d: %+v", job.EvaluationID, result)
			db.Model(&domain.Evaluation{}).
				Where("id = ?", job.EvaluationID).
				Update("status", "failed")
			return
		}

		// Update evaluation dengan hasil
		db.Model(&domain.Evaluation{}).
			Where("id = ?", job.EvaluationID).
			Updates(map[string]interface{}{
				"status":           "completed",
				"cv_match_rate":    cv["match_rate"],
				"cv_feedback":      cv["feedback"],
				"project_score":    project["score"],
				"project_feedback": project["feedback"],
				"overall_summary":  result["overall_summary"],
				"result_json":      &resultStr,
				"updated_at":       time.Now(),
			})

		log.Printf("✅ Worker finished job %d\n", job.EvaluationID)
	})

	// Setup Gin router
	router := gin.Default()
	interfaces.NewHTTPHandler(router, db, rmq)

	log.Println("🚀 Server running on http://localhost:8080")
	if err := router.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
