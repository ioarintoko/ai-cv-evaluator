package domain

import "time"

// Entity definitions (bisa juga dipindah ke /domain nanti)
type Job struct {
	ID          uint   `gorm:"primaryKey"`
	Title       string `gorm:"size:255;not null"`
	Description string `gorm:"type:text;not null"`
	Rubric      string `gorm:"type:json;not null"`
	CreatedAt   time.Time
}
