package chatwoot_model

import "time"

type ChatwootMessageMapping struct {
	WaMessageID       string    `gorm:"primaryKey;size:255"`
	ChatwootMessageID int64     `gorm:"not null;index"`
	ConversationID    int64     `gorm:"not null;index"`
	InstanceID        string    `gorm:"not null;index;size:255"`
	CreatedAt         time.Time
}
