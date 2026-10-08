package models

import "time"

type Message struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	From      uint      `json:"from" gorm:"column:sender_id;index:idx_sender_created,priority:1"`
	To        uint      `json:"to" gorm:"column:receiver_id;index:idx_receiver_created,priority:1"`
	Text      string    `json:"text" gorm:"column:content"`
	CreatedAt time.Time `json:"created_at" gorm:"index:idx_sender_created,priority:2;index:idx_receiver_created,priority:2"`
}
