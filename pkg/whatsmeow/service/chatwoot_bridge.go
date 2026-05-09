package whatsmeow_service

import (
	"strings"

	"go.mau.fi/whatsmeow/proto/waE2E"
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
	var phone string
	if isGroup {
		phone = evt.Info.Chat.String()
	} else {
		phone = evt.Info.Sender.ToNonAD().User
	}

	name := evt.Info.PushName
	waMessageID := evt.Info.ID

	// Extract text content and media URL
	text, _ := chatwoot_service.ExtractMessageContent(evt.Message)

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
		name,
		"", // avatarURL: not available in event context
		text,
		parsedMessageType,
		mediaURL,
		waMessageID,
		evt.Info.IsFromMe,
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
