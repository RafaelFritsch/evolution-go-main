package whatsmeow_service

import (
	"context"
	"fmt"
	"strings"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"

	chatwoot_service "github.com/EvolutionAPI/evolution-go/pkg/chatwoot/service"
)

// processChatwootEvent is called from myEventHandler for every incoming *events.Message.
// It runs in its own goroutine so it never blocks the main event loop.
func (mycli *MyClient) processChatwootEvent(evt *events.Message, parsedMessageType string, dataMap map[string]interface{}) {
	svc := chatwoot_service.GetService()
	if svc == nil {
		return
	}

	instanceName := mycli.Instance.Name

	// --- Task 5.4: message deletion via ProtocolMessage/REVOKE ---
	if proto := evt.Message.GetProtocolMessage(); proto != nil {
		if proto.GetType() == waE2E.ProtocolMessage_REVOKE {
			deletedKey := proto.GetKey()
			if deletedKey != nil && deletedKey.GetID() != "" {
				svc.HandleMessageDeleted(instanceName, deletedKey.GetID())
			}
			return
		}
	}

	// Ignore reaction messages from chatwoot forwarding (they create noise)
	if parsedMessageType == "ignore" || strings.HasPrefix(parsedMessageType, "unknown_protocol_") {
		return
	}

	// Extract phone / group JID
	isGroup := strings.HasSuffix(evt.Info.Chat.String(), "@g.us")
	chatJID := evt.Info.Chat.String()
	senderJID := evt.Info.Sender.String()
	senderName := mycli.resolveChatwootSenderName(evt)
	displayName := senderName
	var phone string
	if isGroup {
		phone = chatJID
		displayName = resolveChatwootGroupName(dataMap, chatJID)
	} else {
		phone = evt.Info.Sender.ToNonAD().User
	}
	waMessageID := evt.Info.ID

	// Extract text content and media URL
	text, mediaType := chatwoot_service.ExtractMessageContent(evt.Message)
	if transcript := chatwoot_service.ExtractAudioTranscript(dataMap, evt.Message); transcript != "" {
		text = fmt.Sprintf("Transcricao do audio:\n%s", transcript)
	}
	if mediaType == "reaction" {
		text = buildChatwootReactionMessage(text, extractQuotedContent(dataMap))
	}

	mediaURL := ""
	if msg, ok := dataMap["Message"].(map[string]interface{}); ok {
		if u, ok := msg["mediaUrl"].(string); ok {
			mediaURL = u
		}
	}

	// --- Task 5.3: outgoing (IsFromMe) messages ---
	svc.SendMessageToConversation(
		instanceName,
		phone,
		displayName,
		"", // avatarURL: not available in event context
		text,
		parsedMessageType,
		mediaURL,
		waMessageID,
		evt.Info.IsFromMe,
		&chatwoot_service.ForwardMessageOptions{
			SenderName: senderName,
			SenderJID:  senderJID,
			Private:    mediaType == "reaction",
		},
	)
}

// triggerChatwootImport is called from myEventHandler on *events.Connected.
// It starts historical data import in background if the instance has chatwoot enabled.
func (mycli *MyClient) triggerChatwootImport() {
	svc := chatwoot_service.GetService()
	if svc == nil {
		return
	}
	instanceName := mycli.Instance.Name
	setting, err := svc.FindChatwoot(instanceName)
	if err != nil || !setting.Enabled {
		return
	}
	if setting.ImportContacts || setting.ImportMessages {
		go svc.ImportHistoricalData(instanceName, setting)
	}
}

func (mycli *MyClient) resolveChatwootSenderName(evt *events.Message) string {
	if evt.Info.PushName != "" {
		return evt.Info.PushName
	}
	if mycli.WAClient == nil || mycli.WAClient.Store == nil || mycli.WAClient.Store.Contacts == nil {
		return evt.Info.Sender.User
	}

	contact, err := mycli.WAClient.Store.Contacts.GetContact(context.Background(), evt.Info.Sender)
	if err != nil {
		return evt.Info.Sender.User
	}
	if contact.PushName != "" {
		return contact.PushName
	}
	if contact.FullName != "" {
		return contact.FullName
	}
	return evt.Info.Sender.User
}

func resolveChatwootGroupName(dataMap map[string]interface{}, fallback string) string {
	groupData, ok := dataMap["groupData"]
	if !ok || groupData == nil {
		return fallback
	}

	switch v := groupData.(type) {
	case *types.GroupInfo:
		if v.GroupName.Name != "" {
			return v.GroupName.Name
		}
	case map[string]interface{}:
		if groupName, ok := v["GroupName"].(map[string]interface{}); ok {
			if name, ok := groupName["Name"].(string); ok && strings.TrimSpace(name) != "" {
				return name
			}
		}
		if groupName, ok := v["groupName"].(map[string]interface{}); ok {
			if name, ok := groupName["name"].(string); ok && strings.TrimSpace(name) != "" {
				return name
			}
		}
	}

	return fallback
}

func extractQuotedContent(dataMap map[string]interface{}) string {
	quoted, ok := dataMap["quoted"].(map[string]interface{})
	if !ok {
		return ""
	}

	msg, ok := quoted["quotedMessage"].(*waE2E.Message)
	if !ok || msg == nil {
		return ""
	}

	text, mediaType := chatwoot_service.ExtractMessageContent(msg)
	if strings.TrimSpace(text) != "" {
		return text
	}
	switch mediaType {
	case "image":
		return "[imagem]"
	case "video":
		return "[video]"
	case "audio":
		return "[audio]"
	case "document":
		return "[documento]"
	default:
		return ""
	}
}

func buildChatwootReactionMessage(emoji, original string) string {
	emoji = strings.TrimSpace(emoji)
	if emoji == "" {
		emoji = "(removida)"
	}
	original = strings.TrimSpace(original)
	if original == "" {
		original = "(mensagem original indisponivel)"
	}
	return fmt.Sprintf("_Reacao: %s na mensagem: %q_", emoji, original)
}
