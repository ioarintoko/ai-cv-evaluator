package domain

import "time"

type Evaluation struct {
	ID              uint    `gorm:"primaryKey"`
	UploadID        uint    `gorm:"not null"`
	JobID           uint    `gorm:"not null"`
	Status          string  `gorm:"type:enum('queued','processing','completed','failed');default:'queued'"`
	CVMatchRate     float64 `gorm:"column:cv_match_rate"`
	CVFeedback      string  `gorm:"type:text"`
	ProjectScore    float64
	ProjectFeedback string  `gorm:"type:text"`
	OverallSummary  string  `gorm:"type:text"`
	ResultJSON      *string `gorm:"type:json"` // pointer biar bisa NULL
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
