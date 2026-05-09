package chatwoot_service

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"go.mau.fi/whatsmeow"
	"gorm.io/gorm"

	chatwoot_client "github.com/EvolutionAPI/evolution-go/pkg/chatwoot/client"
	chatwoot_dto "github.com/EvolutionAPI/evolution-go/pkg/chatwoot/dto"
	chatwoot_model "github.com/EvolutionAPI/evolution-go/pkg/chatwoot/model"
	"github.com/EvolutionAPI/evolution-go/pkg/config"
	logger_wrapper "github.com/EvolutionAPI/evolution-go/pkg/logger"
)

// globalService is the package-level singleton used by the whatsmeow bridge.
var globalService ChatwootService

// Register stores the service singleton for cross-package access.
func Register(svc ChatwootService) { globalService = svc }

// GetService returns the registered singleton (may be nil if not configured).
func GetService() ChatwootService { return globalService }

// ChatwootService defines all operations exposed to other packages.
type ChatwootService interface {
	SetChatwoot(instanceName string, req *chatwoot_dto.SetChatwootRequest) (*chatwoot_model.ChatwootSetting, error)
	FindChatwoot(instanceName string) (*chatwoot_model.ChatwootSetting, error)
	SendMessageToConversation(instanceName, phone, name, avatarURL, content, msgType, mediaURL, waMessageID string, isFromMe bool, opts *ForwardMessageOptions) error
	HandleMessageDeleted(instanceName, waMessageID string) error
	ImportHistoricalData(instanceName string, setting *chatwoot_model.ChatwootSetting)
}

type ForwardMessageOptions struct {
	SenderName string
	SenderJID  string
	Private    bool
}

type chatwootAPI interface {
	CreateContact(payload chatwoot_client.CreateContactPayload) (*chatwoot_client.Contact, error)
	FindContact(query string) ([]chatwoot_client.Contact, error)
	UpdateContact(contactID int64, payload chatwoot_client.CreateContactPayload) (*chatwoot_client.Contact, error)
	MergeContacts(parentID, childID int64) error
	GetConversations(contactID, inboxID int64) ([]chatwoot_client.Conversation, error)
	CreateConversation(payload chatwoot_client.CreateConversationPayload) (*chatwoot_client.Conversation, error)
	ToggleConversationStatus(conversationID int64, status string) error
	CreateMessage(conversationID int64, payload chatwoot_client.CreateMessagePayload) (*chatwoot_client.Message, error)
	DeleteMessage(conversationID, messageID int64) error
	CreateInbox(name, webhookURL string) (*chatwoot_client.Inbox, error)
	GetInboxes() ([]chatwoot_client.Inbox, error)
}

type chatwootService struct {
	db             *gorm.DB
	config         *config.Config
	loggerWrapper  *logger_wrapper.LoggerManager
	conversationMu sync.Map
	clientPointer  map[string]*whatsmeow.Client
	clientFactory  func(setting *chatwoot_model.ChatwootSetting) chatwootAPI
}

func NewChatwootService(db *gorm.DB, cfg *config.Config, lw *logger_wrapper.LoggerManager, clientPointer map[string]*whatsmeow.Client) ChatwootService {
	return &chatwootService{
		db:            db,
		config:        cfg,
		loggerWrapper: lw,
		clientPointer: clientPointer,
		clientFactory: func(setting *chatwoot_model.ChatwootSetting) chatwootAPI {
			return chatwoot_client.NewChatwootClient(setting.URL, setting.Token, setting.AccountID)
		},
	}
}

func (s *chatwootService) log(instanceName string) *logger_wrapper.Logger {
	return s.loggerWrapper.GetLogger(instanceName)
}

func (s *chatwootService) getMutex(key string) *sync.Mutex {
	v, _ := s.conversationMu.LoadOrStore(key, &sync.Mutex{})
	return v.(*sync.Mutex)
}

func (s *chatwootService) newClient(setting *chatwoot_model.ChatwootSetting) chatwootAPI {
	return s.clientFactory(setting)
}

// --- Task 3.2 ---

func (s *chatwootService) SetChatwoot(instanceName string, req *chatwoot_dto.SetChatwootRequest) (*chatwoot_model.ChatwootSetting, error) {
	var setting chatwoot_model.ChatwootSetting
	s.db.Where("instance_id = ?", instanceName).First(&setting)

	ignoreJidsJSON := "[]"
	if len(req.IgnoreJids) > 0 {
		if data, err := json.Marshal(req.IgnoreJids); err == nil {
			ignoreJidsJSON = string(data)
		}
	}

	delimiter := req.SignDelimiter
	if delimiter == "" {
		delimiter = "\n"
	}
	daysLimit := req.DaysLimitImportMessages
	if daysLimit == 0 {
		daysLimit = 7
	}
	nameInbox := req.NameInbox
	if nameInbox == "" {
		nameInbox = instanceName
	}

	setting.InstanceID = instanceName
	setting.Enabled = req.Enabled
	setting.AccountID = req.AccountID
	setting.Token = req.Token
	setting.URL = req.URL
	setting.SignMsg = req.SignMsg
	setting.SignDelimiter = delimiter
	setting.ReopenConversation = req.ReopenConversation
	setting.ConversationPending = req.ConversationPending
	setting.NameInbox = nameInbox
	setting.MergeBrazilContacts = req.MergeBrazilContacts
	setting.ImportContacts = req.ImportContacts
	setting.ImportMessages = req.ImportMessages
	setting.DaysLimitImportMessages = daysLimit
	setting.AutoCreate = req.AutoCreate
	setting.Organization = req.Organization
	setting.Logo = req.Logo
	setting.IgnoreJids = ignoreJidsJSON

	if err := s.db.Save(&setting).Error; err != nil {
		return nil, err
	}

	if req.AutoCreate {
		go func() {
			if err := s.autoCreateInboxAndContact(instanceName, &setting); err != nil {
				s.log(instanceName).LogErrorWithMetadata(
					fmt.Sprintf("[chatwoot][%s] auto-create inbox failed", instanceName),
					map[string]interface{}{
						"instance":   instanceName,
						"account_id": setting.AccountID,
						"inbox_name": setting.NameInbox,
						"error":      err.Error(),
					},
				)
			}
		}()
	}

	return &setting, nil
}

// --- Task 3.3 ---

func (s *chatwootService) FindChatwoot(instanceName string) (*chatwoot_model.ChatwootSetting, error) {
	var setting chatwoot_model.ChatwootSetting
	if err := s.db.Where("instance_id = ?", instanceName).First(&setting).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("chatwoot not configured for instance: %s", instanceName)
		}
		return nil, err
	}
	return &setting, nil
}

// --- Task 3.4 ---

func (s *chatwootService) getOrCreateContact(client chatwootAPI, setting *chatwoot_model.ChatwootSetting, phone, name, avatarURL, identifier string) (int64, error) {
	query := identifier
	if query == "" {
		query = phone
	}

	contacts, err := client.FindContact(query)
	if err != nil {
		return 0, fmt.Errorf("find contact: %w", err)
	}

	if len(contacts) > 0 {
		c := contacts[0]
		client.UpdateContact(c.ID, chatwoot_client.CreateContactPayload{ //nolint:errcheck
			Name:       name,
			Identifier: identifier,
			AvatarURL:  avatarURL,
		})
		if setting.MergeBrazilContacts && len(contacts) > 1 {
			for _, dup := range contacts[1:] {
				client.MergeContacts(c.ID, dup.ID) //nolint:errcheck
			}
		}
		return c.ID, nil
	}

	phoneForCreate := phone
	if phone != "" && !strings.HasPrefix(phone, "+") {
		phoneForCreate = "+" + phone
	}

	created, err := client.CreateContact(chatwoot_client.CreateContactPayload{
		Name:        name,
		PhoneNumber: phoneForCreate,
		Identifier:  identifier,
		AvatarURL:   avatarURL,
	})
	if err != nil {
		return 0, fmt.Errorf("create contact: %w", err)
	}
	return created.ID, nil
}

// --- Task 3.5 ---

func (s *chatwootService) getOrCreateConversation(client chatwootAPI, setting *chatwoot_model.ChatwootSetting, contactID, inboxID int64, instanceName, phone string) (int64, error) {
	mu := s.getMutex(fmt.Sprintf("%s:%s", instanceName, phone))
	mu.Lock()
	defer mu.Unlock()

	convs, err := client.GetConversations(contactID, inboxID)
	if err != nil {
		return 0, fmt.Errorf("get conversations: %w", err)
	}

	for _, conv := range convs {
		if conv.Status == "open" || conv.Status == "pending" {
			return conv.ID, nil
		}
	}

	if setting.ReopenConversation {
		for _, conv := range convs {
			if conv.Status == "resolved" {
				if err := client.ToggleConversationStatus(conv.ID, "open"); err == nil {
					return conv.ID, nil
				}
			}
		}
	}

	status := "open"
	if setting.ConversationPending {
		status = "pending"
	}

	conv, err := client.CreateConversation(chatwoot_client.CreateConversationPayload{
		ContactID: contactID,
		InboxID:   inboxID,
		Status:    status,
	})
	if err != nil {
		return 0, fmt.Errorf("create conversation: %w", err)
	}
	return conv.ID, nil
}

// --- Task 3.6 ---

func (s *chatwootService) autoCreateInboxAndContact(instanceName string, setting *chatwoot_model.ChatwootSetting) error {
	serverURL := os.Getenv("SERVER_URL")
	if serverURL == "" {
		port := os.Getenv("SERVER_PORT")
		if port == "" {
			port = "8080"
		}
		serverURL = fmt.Sprintf("http://localhost:%s", port)
	}
	webhookURL := fmt.Sprintf("%s/chatwoot/webhook/%s", serverURL, instanceName)

	client := s.newClient(setting)

	inboxes, err := client.GetInboxes()
	if err != nil {
		return fmt.Errorf("get inboxes: %w", err)
	}

	var inboxID int64
	for _, inbox := range inboxes {
		if inbox.Name == setting.NameInbox {
			inboxID = inbox.ID
			break
		}
	}

	if inboxID == 0 {
		inbox, err := client.CreateInbox(setting.NameInbox, webhookURL)
		if err != nil {
			return fmt.Errorf("create inbox: %w", err)
		}
		inboxID = inbox.ID
		s.log(instanceName).LogInfoWithMetadata(
			fmt.Sprintf("[chatwoot][%s] inbox created", instanceName),
			map[string]interface{}{
				"instance":   instanceName,
				"inbox_id":   inboxID,
				"inbox_name": setting.NameInbox,
				"webhook":    webhookURL,
			},
		)
	}

	if err := s.db.Model(setting).Update("inbox_id", inboxID).Error; err != nil {
		return fmt.Errorf("save inbox_id: %w", err)
	}
	setting.InboxID = inboxID

	s.log(instanceName).LogInfoWithMetadata(
		fmt.Sprintf("[chatwoot][%s] auto-create inbox completed", instanceName),
		map[string]interface{}{
			"instance":   instanceName,
			"inbox_id":   inboxID,
			"inbox_name": setting.NameInbox,
			"webhook":    webhookURL,
		},
	)
	return nil
}

// --- Task 3.7 ---

func (s *chatwootService) resolveInboxID(client chatwootAPI, setting *chatwoot_model.ChatwootSetting, instanceName string) (int64, error) {
	if setting.InboxID != 0 {
		return setting.InboxID, nil
	}
	inboxes, err := client.GetInboxes()
	if err != nil {
		return 0, err
	}
	for _, inbox := range inboxes {
		if inbox.Name == setting.NameInbox {
			s.db.Model(setting).Update("inbox_id", inbox.ID)
			setting.InboxID = inbox.ID
			return inbox.ID, nil
		}
	}
	return 0, fmt.Errorf("inbox %q not found in chatwoot", setting.NameInbox)
}

func (s *chatwootService) isIgnored(setting *chatwoot_model.ChatwootSetting, jid string) bool {
	if setting.IgnoreJids == "" || setting.IgnoreJids == "[]" {
		return false
	}
	var list []string
	if json.Unmarshal([]byte(setting.IgnoreJids), &list) != nil {
		return false
	}
	for _, v := range list {
		if v == jid {
			return true
		}
	}
	return false
}

func (s *chatwootService) SendMessageToConversation(instanceName, phone, name, avatarURL, content, msgType, mediaURL, waMessageID string, isFromMe bool, opts *ForwardMessageOptions) error {
	setting, err := s.FindChatwoot(instanceName)
	if err != nil || !setting.Enabled {
		return nil
	}
	if s.isIgnored(setting, phone) {
		return nil
	}
	if opts != nil && opts.SenderJID != "" && s.isIgnored(setting, opts.SenderJID) {
		return nil
	}

	isGroup := strings.Contains(phone, "@g.us")
	client := s.newClient(setting)

	inboxID, err := s.resolveInboxID(client, setting, instanceName)
	if err != nil {
		s.log(instanceName).LogErrorWithMetadata(
			fmt.Sprintf("[chatwoot][%s] resolve inbox failed", instanceName),
			map[string]interface{}{
				"instance": instanceName,
				"phone":    phone,
				"error":    err.Error(),
			},
		)
		return err
	}

	identifier := phone
	contactPhone := ""
	if !isGroup {
		rawPhone := strings.TrimSuffix(phone, "@s.whatsapp.net")
		rawPhone = strings.TrimSuffix(rawPhone, "@c.us")
		if !strings.HasPrefix(rawPhone, "+") {
			rawPhone = "+" + rawPhone
		}
		contactPhone = rawPhone
	}

	contactID, err := s.getOrCreateContact(client, setting, contactPhone, name, avatarURL, identifier)
	if err != nil {
		s.log(instanceName).LogErrorWithMetadata(
			fmt.Sprintf("[chatwoot][%s] contact lookup/create failed", instanceName),
			map[string]interface{}{
				"instance":   instanceName,
				"phone":      phone,
				"identifier": identifier,
				"error":      err.Error(),
			},
		)
		return err
	}

	conversationID, err := s.getOrCreateConversation(client, setting, contactID, inboxID, instanceName, phone)
	if err != nil {
		s.log(instanceName).LogErrorWithMetadata(
			fmt.Sprintf("[chatwoot][%s] conversation lookup/create failed", instanceName),
			map[string]interface{}{
				"instance": instanceName,
				"phone":    phone,
				"inbox_id": inboxID,
				"error":    err.Error(),
			},
		)
		return err
	}

	chatwootMsgType := "incoming"
	if isFromMe {
		chatwootMsgType = "outgoing"
	}

	msgContent := strings.TrimSpace(content)
	if msgContent == "" {
		msgContent = defaultContentForMessageType(msgType)
	}

	senderName := ""
	isPrivate := false
	if opts != nil {
		senderName = strings.TrimSpace(opts.SenderName)
		isPrivate = opts.Private
	}

	switch {
	case isGroup && !isFromMe && senderName != "":
		msgContent = fmt.Sprintf("*%s:* %s", senderName, msgContent)
	case setting.SignMsg && !isFromMe && name != "":
		msgContent = fmt.Sprintf("%s%s%s", name, setting.SignDelimiter, msgContent)
	}
	msgContent = FormatWhatsAppToMarkdown(msgContent)

	payload := chatwoot_client.CreateMessagePayload{
		Content:     msgContent,
		MessageType: chatwootMsgType,
		Private:     isPrivate,
		ContentType: "text",
	}
	if mediaURL != "" {
		payload.Attachments = []chatwoot_client.MessageAttachment{
			{ResourceType: "file", ResourceURL: mediaURL},
		}
	}

	msg, err := client.CreateMessage(conversationID, payload)
	if err != nil {
		s.log(instanceName).LogErrorWithMetadata(
			fmt.Sprintf("[chatwoot][%s] create message failed", instanceName),
			map[string]interface{}{
				"instance":        instanceName,
				"conversation_id": conversationID,
				"wa_message_id":   waMessageID,
				"message_type":    chatwootMsgType,
				"error":           err.Error(),
			},
		)
		return err
	}

	if msg != nil && waMessageID != "" {
		mapping := chatwoot_model.ChatwootMessageMapping{
			WaMessageID:       waMessageID,
			ChatwootMessageID: msg.ID,
			ConversationID:    conversationID,
			InstanceID:        instanceName,
			CreatedAt:         time.Now(),
		}
		s.db.Save(&mapping) //nolint:errcheck
	}

	s.log(instanceName).LogInfoWithMetadata(
		fmt.Sprintf("[chatwoot][%s] message forwarded", instanceName),
		map[string]interface{}{
			"instance":        instanceName,
			"conversation_id": conversationID,
			"chatwoot_msg_id": msg.ID,
			"wa_message_id":   waMessageID,
			"message_type":    chatwootMsgType,
			"is_group":        isGroup,
			"private":         isPrivate,
		},
	)
	return nil
}

func defaultContentForMessageType(msgType string) string {
	switch {
	case strings.HasPrefix(msgType, "image"):
		return "[imagem]"
	case strings.HasPrefix(msgType, "video"), strings.HasPrefix(msgType, "round video"):
		return "[video]"
	case strings.HasPrefix(msgType, "audio"):
		return "[audio]"
	case strings.HasPrefix(msgType, "document"):
		return "[documento]"
	case strings.HasPrefix(msgType, "sticker"):
		return "[figurinha]"
	case strings.HasPrefix(msgType, "location"), strings.HasPrefix(msgType, "live location"):
		return "[localizacao]"
	case strings.HasPrefix(msgType, "contact"):
		return "[contato]"
	case strings.HasPrefix(msgType, "poll"):
		return "[enquete]"
	default:
		return ""
	}
}

// --- Task 3.8 ---

func (s *chatwootService) SendOutgoingMessage(instanceName string, conversationID int64, content string) error {
	setting, err := s.FindChatwoot(instanceName)
	if err != nil {
		return err
	}
	client := s.newClient(setting)
	_, err = client.CreateMessage(conversationID, chatwoot_client.CreateMessagePayload{
		Content:     content,
		MessageType: "outgoing",
		Private:     false,
		ContentType: "text",
	})
	return err
}

// --- Task 5.4 ---

func (s *chatwootService) HandleMessageDeleted(instanceName, waMessageID string) error {
	var mapping chatwoot_model.ChatwootMessageMapping
	if err := s.db.Where("wa_message_id = ? AND instance_id = ?", waMessageID, instanceName).First(&mapping).Error; err != nil {
		return nil
	}

	setting, err := s.FindChatwoot(instanceName)
	if err != nil || !setting.Enabled {
		return nil
	}

	client := s.newClient(setting)
	if err := client.DeleteMessage(mapping.ConversationID, mapping.ChatwootMessageID); err != nil {
		s.log(instanceName).LogErrorWithMetadata(
			fmt.Sprintf("[chatwoot][%s] delete mapped message failed", instanceName),
			map[string]interface{}{
				"instance":            instanceName,
				"conversation_id":     mapping.ConversationID,
				"chatwoot_message_id": mapping.ChatwootMessageID,
				"wa_message_id":       waMessageID,
				"error":               err.Error(),
			},
		)
		return err
	}

	s.db.Delete(&mapping) //nolint:errcheck
	s.log(instanceName).LogInfoWithMetadata(
		fmt.Sprintf("[chatwoot][%s] deleted mapped chatwoot message", instanceName),
		map[string]interface{}{
			"instance":            instanceName,
			"conversation_id":     mapping.ConversationID,
			"chatwoot_message_id": mapping.ChatwootMessageID,
			"wa_message_id":       waMessageID,
		},
	)
	return nil
}
