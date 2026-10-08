package models

import "time"

type User struct {
	ID           uint   `gorm:"primaryKey"`
	Username     string `gorm:"type:varchar(191);uniqueIndex;not null"`
	PasswordHash string `gorm:"type:varchar(255);not null"`
	CreatedAt    time.Time
}
