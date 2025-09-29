package infrastructure

import (
	"cv-evaluator/domain"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func NewMySQLConnection() *gorm.DB {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		log.Fatal("DB_DSN is not set in environment")
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	// Auto migrate schema
	err = db.AutoMigrate(&domain.Job{}, &domain.Upload{}, &domain.Evaluation{})
	if err != nil {
		log.Fatalf("failed to migrate database: %v", err)
	}

	// Seed initial jobs
	seedJobs(db)

	fmt.Println("✅ Connected to MySQL and migrated schema")
	return db
}

func seedJobs(db *gorm.DB) {
	var count int64
	if err := db.Model(&domain.Job{}).Count(&count).Error; err != nil {
		log.Fatalf("failed to count jobs: %v", err)
	}

	if count > 0 {
		return // sudah ada data, tidak insert lagi
	}

	// rubric untuk job kedua
	rubric := map[string]interface{}{
		"experience": map[string]interface{}{
			"weight":   25,
			"criteria": "Years of backend development, project complexity, system scaling experience",
		},
		"achievements": map[string]interface{}{
			"weight":   20,
			"criteria": "Impactful projects, performance improvements, AI feature implementations",
		},
		"cultural_fit": map[string]interface{}{
			"weight":   15,
			"criteria": "Communication, learning attitude, remote work capability",
		},
		"technical_skills": map[string]interface{}{
			"weight":   40,
			"criteria": "Backend languages (Go, PHP), databases (MySQL), message queues (RabbitMQ), API design, AI integration",
		},
	}
	rubricJSON, _ := json.Marshal(rubric)

	jobs := []domain.Job{
		{
			Title:       "Test Job",
			Description: "Backend evaluation system with AI",
			Rubric:      "{}",
			CreatedAt:   time.Now(),
		},
		{
			Title: "Test Job",
			Description: "Product Engineer (Backend) with focus on Go, PHP, MySQL, RabbitMQ, AI/LLM integration, " +
				"and building scalable backend systems. Experience with RESTful APIs, database management, cloud technologies, " +
				"and AI-powered features is required.",
			Rubric:    string(rubricJSON),
			CreatedAt: time.Now(),
		},
	}

	if err := db.Create(&jobs).Error; err != nil {
		log.Fatalf("failed to seed jobs: %v", err)
	}

	fmt.Println("✅ Seeded initial jobs")
}
