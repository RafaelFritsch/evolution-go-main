package chatwoot_service

import (
	"strings"
	"testing"

	chatwoot_client "github.com/EvolutionAPI/evolution-go/pkg/chatwoot/client"
	chatwoot_dto "github.com/EvolutionAPI/evolution-go/pkg/chatwoot/dto"
	chatwoot_model "github.com/EvolutionAPI/evolution-go/pkg/chatwoot/model"
	"github.com/EvolutionAPI/evolution-go/pkg/config"
	logger_wrapper "github.com/EvolutionAPI/evolution-go/pkg/logger"
	"go.mau.fi/whatsmeow"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type mockChatwootClient struct {
	findContactFn              func(query string) ([]chatwoot_client.Contact, error)
	updateContactFn            func(contactID int64, payload chatwoot_client.CreateContactPayload) (*chatwoot_client.Contact, error)
	createContactFn            func(payload chatwoot_client.CreateContactPayload) (*chatwoot_client.Contact, error)
	mergeContactsFn            func(parentID, childID int64) error
	getConversationsFn         func(contactID, inboxID int64) ([]chatwoot_client.Conversation, error)
	toggleConversationStatusFn func(conversationID int64, status string) error
	createConversationFn       func(payload chatwoot_client.CreateConversationPayload) (*chatwoot_client.Conversation, error)
	createMessageFn            func(conversationID int64, payload chatwoot_client.CreateMessagePayload) (*chatwoot_client.Message, error)
	deleteMessageFn            func(conversationID, messageID int64) error
	createInboxFn              func(name, webhookURL string) (*chatwoot_client.Inbox, error)
	getInboxesFn               func() ([]chatwoot_client.Inbox, error)
	createContactCalls         int
	createConversationCalls    int
	toggleConversationCalls    int
}

func (m *mockChatwootClient) CreateContact(payload chatwoot_client.CreateContactPayload) (*chatwoot_client.Contact, error) {
	m.createContactCalls++
	if m.createContactFn != nil {
		return m.createContactFn(payload)
	}
	return &chatwoot_client.Contact{ID: 1}, nil
}

func (m *mockChatwootClient) FindContact(query string) ([]chatwoot_client.Contact, error) {
	if m.findContactFn != nil {
		return m.findContactFn(query)
	}
	return nil, nil
}

func (m *mockChatwootClient) UpdateContact(contactID int64, payload chatwoot_client.CreateContactPayload) (*chatwoot_client.Contact, error) {
	if m.updateContactFn != nil {
		return m.updateContactFn(contactID, payload)
	}
	return &chatwoot_client.Contact{ID: contactID}, nil
}

func (m *mockChatwootClient) MergeContacts(parentID, childID int64) error {
	if m.mergeContactsFn != nil {
		return m.mergeContactsFn(parentID, childID)
	}
	return nil
}

func (m *mockChatwootClient) GetConversations(contactID, inboxID int64) ([]chatwoot_client.Conversation, error) {
	if m.getConversationsFn != nil {
		return m.getConversationsFn(contactID, inboxID)
	}
	return nil, nil
}

func (m *mockChatwootClient) CreateConversation(payload chatwoot_client.CreateConversationPayload) (*chatwoot_client.Conversation, error) {
	m.createConversationCalls++
	if m.createConversationFn != nil {
		return m.createConversationFn(payload)
	}
	return &chatwoot_client.Conversation{ID: 1}, nil
}

func (m *mockChatwootClient) ToggleConversationStatus(conversationID int64, status string) error {
	m.toggleConversationCalls++
	if m.toggleConversationStatusFn != nil {
		return m.toggleConversationStatusFn(conversationID, status)
	}
	return nil
}

func (m *mockChatwootClient) CreateMessage(conversationID int64, payload chatwoot_client.CreateMessagePayload) (*chatwoot_client.Message, error) {
	if m.createMessageFn != nil {
		return m.createMessageFn(conversationID, payload)
	}
	return &chatwoot_client.Message{ID: 1}, nil
}

func (m *mockChatwootClient) DeleteMessage(conversationID, messageID int64) error {
	if m.deleteMessageFn != nil {
		return m.deleteMessageFn(conversationID, messageID)
	}
	return nil
}

func (m *mockChatwootClient) CreateInbox(name, webhookURL string) (*chatwoot_client.Inbox, error) {
	if m.createInboxFn != nil {
		return m.createInboxFn(name, webhookURL)
	}
	return &chatwoot_client.Inbox{ID: 1, Name: name}, nil
}

func (m *mockChatwootClient) GetInboxes() ([]chatwoot_client.Inbox, error) {
	if m.getInboxesFn != nil {
		return m.getInboxesFn()
	}
	return nil, nil
}

func newTestService(t *testing.T, client chatwootAPI) *chatwootService {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	if err := db.AutoMigrate(&chatwoot_model.ChatwootSetting{}, &chatwoot_model.ChatwootMessageMapping{}); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}

	cfg := &config.Config{
		DatabaseSaveMessages: true,
		LogDirectory:         t.TempDir(),
		LogMaxSize:           1,
		LogMaxBackups:        1,
		LogMaxAge:            1,
	}

	svc := &chatwootService{
		db:            db,
		config:        cfg,
		loggerWrapper: logger_wrapper.NewLoggerManager(cfg),
		clientPointer: map[string]*whatsmeow.Client{},
	}

	svc.clientFactory = func(setting *chatwoot_model.ChatwootSetting) chatwootAPI {
		if client != nil {
			return client
		}
		return &mockChatwootClient{}
	}

	return svc
}

func TestSetChatwootCreatesNewRecord(t *testing.T) {
	svc := newTestService(t, nil)

	setting, err := svc.SetChatwoot("sales", &chatwoot_dto.SetChatwootRequest{
		Enabled:        true,
		AccountID:      "7",
		Token:          "secret-token",
		URL:            "https://chatwoot.example.com",
		SignMsg:        true,
		IgnoreJids:     []string{"120363000000@g.us"},
		ImportContacts: true,
	})
	if err != nil {
		t.Fatalf("SetChatwoot returned error: %v", err)
	}

	if setting.InstanceID != "sales" {
		t.Fatalf("unexpected instance id: %s", setting.InstanceID)
	}
	if setting.NameInbox != "sales" {
		t.Fatalf("expected default inbox name to be instance name, got %q", setting.NameInbox)
	}
	if setting.SignDelimiter != "\n" {
		t.Fatalf("expected default sign delimiter, got %q", setting.SignDelimiter)
	}
	if setting.DaysLimitImportMessages != 7 {
		t.Fatalf("expected default import window 7, got %d", setting.DaysLimitImportMessages)
	}
	if !strings.Contains(setting.IgnoreJids, "120363000000@g.us") {
		t.Fatalf("expected ignoreJids to be persisted as json, got %q", setting.IgnoreJids)
	}
}

func TestSetChatwootUpdatesExistingRecord(t *testing.T) {
	svc := newTestService(t, nil)

	if _, err := svc.SetChatwoot("sales", &chatwoot_dto.SetChatwootRequest{
		Enabled:   true,
		AccountID: "1",
		Token:     "old-token",
		URL:       "https://old.example.com",
		NameInbox: "legacy",
	}); err != nil {
		t.Fatalf("initial SetChatwoot returned error: %v", err)
	}

	updated, err := svc.SetChatwoot("sales", &chatwoot_dto.SetChatwootRequest{
		Enabled:                 false,
		AccountID:               "2",
		Token:                   "new-token",
		URL:                     "https://new.example.com",
		NameInbox:               "modern",
		ReopenConversation:      true,
		DaysLimitImportMessages: 21,
	})
	if err != nil {
		t.Fatalf("update SetChatwoot returned error: %v", err)
	}

	var count int64
	if err := svc.db.Model(&chatwoot_model.ChatwootSetting{}).Where("instance_id = ?", "sales").Count(&count).Error; err != nil {
		t.Fatalf("count settings: %v", err)
	}

	if count != 1 {
		t.Fatalf("expected a single row after update, got %d", count)
	}
	if updated.AccountID != "2" || updated.URL != "https://new.example.com" {
		t.Fatalf("existing record was not updated correctly: %+v", updated)
	}
	if updated.NameInbox != "modern" || !updated.ReopenConversation || updated.DaysLimitImportMessages != 21 {
		t.Fatalf("updated fields not persisted correctly: %+v", updated)
	}
}

func TestFindChatwootReturnsErrorWhenMissing(t *testing.T) {
	svc := newTestService(t, nil)

	_, err := svc.FindChatwoot("missing")
	if err == nil {
		t.Fatal("expected error for missing chatwoot config")
	}
	if !strings.Contains(err.Error(), "chatwoot not configured") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetOrCreateContactReturnsExistingContactWithoutDuplicating(t *testing.T) {
	mockClient := &mockChatwootClient{
		findContactFn: func(query string) ([]chatwoot_client.Contact, error) {
			return []chatwoot_client.Contact{{ID: 42, Name: "Maria", Identifier: "5511999999999@s.whatsapp.net"}}, nil
		},
	}
	svc := newTestService(t, mockClient)

	contactID, err := svc.getOrCreateContact(mockClient, &chatwoot_model.ChatwootSetting{}, "5511999999999", "Maria", "", "5511999999999@s.whatsapp.net")
	if err != nil {
		t.Fatalf("getOrCreateContact returned error: %v", err)
	}

	if contactID != 42 {
		t.Fatalf("expected existing contact id 42, got %d", contactID)
	}
	if mockClient.createContactCalls != 0 {
		t.Fatalf("expected no contact creation for existing record, got %d calls", mockClient.createContactCalls)
	}
}

func TestGetOrCreateConversationReopensResolvedConversation(t *testing.T) {
	mockClient := &mockChatwootClient{
		getConversationsFn: func(contactID, inboxID int64) ([]chatwoot_client.Conversation, error) {
			return []chatwoot_client.Conversation{
				{ID: 55, Status: "resolved", InboxID: inboxID},
			}, nil
		},
	}
	svc := newTestService(t, mockClient)

	conversationID, err := svc.getOrCreateConversation(mockClient, &chatwoot_model.ChatwootSetting{
		ReopenConversation: true,
	}, 99, 7, "sales", "5511999999999")
	if err != nil {
		t.Fatalf("getOrCreateConversation returned error: %v", err)
	}

	if conversationID != 55 {
		t.Fatalf("expected resolved conversation to be reopened, got %d", conversationID)
	}
	if mockClient.toggleConversationCalls != 1 {
		t.Fatalf("expected one toggle status call, got %d", mockClient.toggleConversationCalls)
	}
	if mockClient.createConversationCalls != 0 {
		t.Fatalf("expected no new conversation creation, got %d calls", mockClient.createConversationCalls)
	}
}
