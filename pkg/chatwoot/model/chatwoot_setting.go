package chatwoot_model

import "time"

type ChatwootSetting struct {
	ID                      uint   `gorm:"primaryKey;autoIncrement"`
	InstanceID              string `gorm:"not null;index;size:255"`
	Enabled                 bool   `gorm:"default:false"`
	AccountID               string `gorm:"not null;size:255"`
	Token                   string `gorm:"not null;size:255"`
	URL                     string `gorm:"not null;size:500"`
	SignMsg                 bool   `gorm:"default:false"`
	SignDelimiter           string `gorm:"size:10"`
	ReopenConversation      bool   `gorm:"default:false"`
	ConversationPending     bool   `gorm:"default:false"`
	NameInbox               string `gorm:"size:255"`
	InboxID                 int64  `gorm:"default:0"`
	MergeBrazilContacts     bool   `gorm:"default:false"`
	ImportContacts          bool   `gorm:"default:false"`
	ImportMessages          bool   `gorm:"default:false"`
	DaysLimitImportMessages int    `gorm:"default:7"`
	AutoCreate              bool   `gorm:"default:false"`
	Organization            string `gorm:"size:255"`
	Logo                    string `gorm:"size:500"`
	IgnoreJids              string `gorm:"type:text"`
	ImportingAt             *time.Time
	CreatedAt               time.Time
	UpdatedAt               time.Time
}
