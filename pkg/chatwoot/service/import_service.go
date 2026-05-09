package chatwoot_service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"

	chatwoot_client "github.com/EvolutionAPI/evolution-go/pkg/chatwoot/client"
	chatwoot_model "github.com/EvolutionAPI/evolution-go/pkg/chatwoot/model"
	message_model "github.com/EvolutionAPI/evolution-go/pkg/message/model"
)

// ImportHistoricalData implements ChatwootService.
// Protected by importing_at: skips if another import ran within the last hour.
func (s *chatwootService) ImportHistoricalData(instanceName string, setting *chatwoot_model.ChatwootSetting) {
	if setting.ImportingAt != nil && time.Since(*setting.ImportingAt) < time.Hour {
		s.log(instanceName).LogInfo("[chatwoot][%s] import already running, skipping", instanceName)
		return
	}

	now := time.Now()
	s.db.Model(setting).Update("importing_at", &now)      //nolint:errcheck
	defer s.db.Model(setting).Update("importing_at", nil) //nolint:errcheck

	s.log(instanceName).LogInfoWithMetadata(
		fmt.Sprintf("[chatwoot][%s] starting historical import", instanceName),
		map[string]interface{}{
			"instance":         instanceName,
			"import_contacts":  setting.ImportContacts,
			"import_messages":  setting.ImportMessages,
			"days_limit":       setting.DaysLimitImportMessages,
			"configured_inbox": setting.NameInbox,
		},
	)

	client := s.newClient(setting)

	inboxID, err := s.resolveInboxID(client, setting, instanceName)
	if err != nil {
		s.log(instanceName).LogErrorWithMetadata(
			fmt.Sprintf("[chatwoot][%s] resolve inbox failed during historical import", instanceName),
			map[string]interface{}{
				"instance": instanceName,
				"error":    err.Error(),
			},
		)
		return
	}

	contactsImported := 0
	if setting.ImportContacts {
		waClient := s.clientPointer[instanceName]
		if waClient == nil {
			s.log(instanceName).LogWarn("[chatwoot][%s] no active whatsapp client for contact import", instanceName)
		} else {
			n, err := s.importContacts(client, setting, waClient)
			if err != nil {
				s.log(instanceName).LogErrorWithMetadata(
					fmt.Sprintf("[chatwoot][%s] contact import failed", instanceName),
					map[string]interface{}{
						"instance": instanceName,
						"inbox_id": inboxID,
						"error":    err.Error(),
					},
				)
			} else {
				contactsImported = n
			}
		}
	}

	conversationsImported := 0
	if setting.ImportMessages {
		n, err := s.importMessages(client, setting, inboxID)
		if err != nil {
			s.log(instanceName).LogErrorWithMetadata(
				fmt.Sprintf("[chatwoot][%s] message import failed", instanceName),
				map[string]interface{}{
					"instance": instanceName,
					"inbox_id": inboxID,
					"error":    err.Error(),
				},
			)
		} else {
			conversationsImported = n
		}
	}

	s.log(instanceName).LogInfoWithMetadata(
		fmt.Sprintf("[chatwoot][%s] historical import finished", instanceName),
		map[string]interface{}{
			"instance":                instanceName,
			"contacts_imported":       contactsImported,
			"conversations_imported":  conversationsImported,
			"messages_import_enabled": setting.ImportMessages,
			"contacts_import_enabled": setting.ImportContacts,
			"days_limit":              setting.DaysLimitImportMessages,
		},
	)
}

// importContacts reads all contacts from the whatsmeow store and creates them in Chatwoot.
// Processed in batches of 100 with 100ms sleep between batches to avoid rate-limiting.
func (s *chatwootService) importContacts(
	client chatwootAPI,
	setting *chatwoot_model.ChatwootSetting,
	waClient *whatsmeow.Client,
) (int, error) {
	contacts, err := waClient.Store.Contacts.GetAllContacts(context.Background())
	if err != nil {
		return 0, fmt.Errorf("get contacts from store: %w", err)
	}

	type entry struct {
		jid  string
		name string
	}

	var entries []entry
	for jid, contact := range contacts {
		jidStr := jid.String()
		if !strings.HasSuffix(jidStr, "@s.whatsapp.net") {
			continue
		}
		name := contact.FullName
		if name == "" {
			name = contact.PushName
		}
		if name == "" {
			name = contact.FirstName
		}
		if name == "" {
			continue
		}
		entries = append(entries, entry{jid: jidStr, name: name})
	}

	const batchSize = 100
	imported := 0

	for i := 0; i < len(entries); i += batchSize {
		end := i + batchSize
		if end > len(entries) {
			end = len(entries)
		}

		for _, e := range entries[i:end] {
			phone := strings.TrimSuffix(e.jid, "@s.whatsapp.net")
			if _, err := s.getOrCreateContact(client, setting, phone, e.name, "", e.jid); err != nil {
				s.log(setting.InstanceID).LogErrorWithMetadata(
					fmt.Sprintf("[chatwoot][%s] import contact failed", setting.InstanceID),
					map[string]interface{}{
						"instance": setting.InstanceID,
						"jid":      e.jid,
						"error":    err.Error(),
					},
				)
				continue
			}
			imported++
		}

		if end < len(entries) {
			time.Sleep(100 * time.Millisecond)
		}
	}

	return imported, nil
}

// importMessages ensures a Chatwoot conversation exists for each unique contact
// that has a local message within the DaysLimitImportMessages window.
// Message content is not stored in the local DB — a private summary note is posted instead.
func (s *chatwootService) importMessages(
	client chatwootAPI,
	setting *chatwoot_model.ChatwootSetting,
	inboxID int64,
) (int, error) {
	if !s.config.DatabaseSaveMessages {
		s.log(setting.InstanceID).LogWarn("[chatwoot][%s] DATABASE_SAVE_MESSAGES=false — message import skipped", setting.InstanceID)
		return 0, nil
	}

	cutoff := time.Now().AddDate(0, 0, -setting.DaysLimitImportMessages)
	cutoffStr := cutoff.Format("2006-01-02 15:04:05")

	var messages []message_model.Message
	if err := s.db.Where("timestamp >= ? AND source != ''", cutoffStr).
		Order("timestamp asc").
		Find(&messages).Error; err != nil {
		return 0, fmt.Errorf("query messages: %w", err)
	}

	// Count messages per phone to include in the summary note.
	phoneCounts := map[string]int{}
	phoneFirst := map[string]string{} // phone → first timestamp in window
	for _, msg := range messages {
		if msg.Source == "" {
			continue
		}
		phoneCounts[msg.Source]++
		if _, ok := phoneFirst[msg.Source]; !ok {
			phoneFirst[msg.Source] = msg.Timestamp
		}
	}

	imported := 0
	for phone, count := range phoneCounts {
		contactID, err := s.getOrCreateContact(client, setting, phone, phone, "", "")
		if err != nil {
			s.log(setting.InstanceID).LogErrorWithMetadata(
				fmt.Sprintf("[chatwoot][%s] import messages contact step failed", setting.InstanceID),
				map[string]interface{}{
					"instance": setting.InstanceID,
					"phone":    phone,
					"error":    err.Error(),
				},
			)
			continue
		}

		convID, err := s.getOrCreateConversation(client, setting, contactID, inboxID, setting.InstanceID, phone)
		if err != nil {
			s.log(setting.InstanceID).LogErrorWithMetadata(
				fmt.Sprintf("[chatwoot][%s] import messages conversation step failed", setting.InstanceID),
				map[string]interface{}{
					"instance": setting.InstanceID,
					"phone":    phone,
					"inbox_id": inboxID,
					"error":    err.Error(),
				},
			)
			continue
		}

		// Post a private retroactive note so agents know there's prior history.
		ts, err := time.Parse("2006-01-02 15:04:05", phoneFirst[phone])
		if err != nil {
			ts = cutoff
		}
		tsUnix := ts.Unix()
		content := fmt.Sprintf("[Historical import — %d messages in the last %d days]",
			count, setting.DaysLimitImportMessages)
		client.CreateMessage(convID, chatwoot_client.CreateMessagePayload{ //nolint:errcheck
			Content:     content,
			MessageType: "incoming",
			Private:     true,
			ContentType: "text",
			CreatedAt:   &tsUnix,
		})

		imported++
	}

	return imported, nil
}
