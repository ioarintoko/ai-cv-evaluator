package domain

import "time"

type Upload struct {
	ID             uint   `gorm:"primaryKey"`
	CandidateName  string `gorm:"size:255"`
	CandidateEmail string `gorm:"size:255"`
	CVText         string `gorm:"type:longtext;not null"`
	ProjectText    string `gorm:"type:longtext;not null"`
	CreatedAt      time.Time
}
