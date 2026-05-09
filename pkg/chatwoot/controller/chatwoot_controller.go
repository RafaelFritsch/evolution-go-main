package chatwoot_controller

import (
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	chatwoot_dto "github.com/EvolutionAPI/evolution-go/pkg/chatwoot/dto"
	chatwoot_service "github.com/EvolutionAPI/evolution-go/pkg/chatwoot/service"
	instance_repository "github.com/EvolutionAPI/evolution-go/pkg/instance/repository"
	send_service "github.com/EvolutionAPI/evolution-go/pkg/sendMessage/service"
)

type ChatwootController struct {
	chatwootService    chatwoot_service.ChatwootService
	sendService        send_service.SendService
	instanceRepository instance_repository.InstanceRepository
}

func NewController(
	svc chatwoot_service.ChatwootService,
	sendSvc send_service.SendService,
	instanceRepo instance_repository.InstanceRepository,
) *ChatwootController {
	return &ChatwootController{
		chatwootService:    svc,
		sendService:        sendSvc,
		instanceRepository: instanceRepo,
	}
}

// SetChatwoot godoc
// @Summary      Configure Chatwoot integration for an instance
// @Tags         Chatwoot
// @Accept       json
// @Produce      json
// @Param        instance  path      string                          true  "Instance name"
// @Param        body      body      chatwoot_dto.SetChatwootRequest true  "Chatwoot settings"
// @Success      200       {object}  chatwoot_dto.ChatwootResponse
// @Failure      400       {object}  gin.H
// @Failure      500       {object}  gin.H
// @Router       /chatwoot/set/{instance} [post]
func (ctrl *ChatwootController) SetChatwoot(c *gin.Context) {
	instanceName := c.Param("instance")

	var req chatwoot_dto.SetChatwootRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	setting, err := ctrl.chatwootService.SetChatwoot(instanceName, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"instanceId": setting.InstanceID,
		"enabled":    setting.Enabled,
		"accountId":  setting.AccountID,
		"url":        setting.URL,
		"nameInbox":  setting.NameInbox,
		"inboxId":    setting.InboxID,
	})
}

// FindChatwoot godoc
// @Summary      Get Chatwoot configuration for an instance
// @Tags         Chatwoot
// @Produce      json
// @Param        instance  path      string  true  "Instance name"
// @Success      200       {object}  chatwoot_dto.ChatwootResponse
// @Failure      404       {object}  gin.H
// @Router       /chatwoot/find/{instance} [get]
func (ctrl *ChatwootController) FindChatwoot(c *gin.Context) {
	instanceName := c.Param("instance")

	setting, err := ctrl.chatwootService.FindChatwoot(instanceName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"instanceId":              setting.InstanceID,
		"enabled":                 setting.Enabled,
		"accountId":               setting.AccountID,
		"url":                     setting.URL,
		"signMsg":                 setting.SignMsg,
		"signDelimiter":           setting.SignDelimiter,
		"reopenConversation":      setting.ReopenConversation,
		"conversationPending":     setting.ConversationPending,
		"nameInbox":               setting.NameInbox,
		"inboxId":                 setting.InboxID,
		"mergeBrazilContacts":     setting.MergeBrazilContacts,
		"importContacts":          setting.ImportContacts,
		"importMessages":          setting.ImportMessages,
		"daysLimitImportMessages": setting.DaysLimitImportMessages,
		"autoCreate":              setting.AutoCreate,
		"organization":            setting.Organization,
		"logo":                    setting.Logo,
	})
}

// ReceiveWebhook handles incoming Chatwoot webhook events and dispatches them to WhatsApp.
//
// @Summary      Receive webhook from Chatwoot
// @Tags         Chatwoot
// @Accept       json
// @Produce      json
// @Param        instance  path      string                               true  "Instance name"
// @Param        body      body      chatwoot_dto.ChatwootWebhookPayload  true  "Webhook payload"
// @Success      200       {object}  gin.H
// @Router       /chatwoot/webhook/{instance} [post]
func (ctrl *ChatwootController) ReceiveWebhook(c *gin.Context) {
	instanceName := c.Param("instance")

	var payload chatwoot_dto.ChatwootWebhookPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusOK, gin.H{"status": "ignored"})
		return
	}

	// Task 7.1: route by event type
	switch payload.Event {
	case "message_created":
		// Only outgoing, non-private agent messages are forwarded to WhatsApp.
		if payload.MessageType != "outgoing" || payload.Private {
			c.JSON(http.StatusOK, gin.H{"status": "ignored"})
			return
		}
		// Task 7.5: #inbox_whatsapp: special command
		if strings.HasPrefix(payload.Content, "#inbox_whatsapp:") {
			go ctrl.handleInboxWhatsappCommand(instanceName, payload)
			c.JSON(http.StatusOK, gin.H{"status": "command_received"})
			return
		}
		// Tasks 7.2 / 7.3 / 7.4: dispatch text or media to WhatsApp
		go ctrl.dispatchToWhatsApp(instanceName, payload)
		c.JSON(http.StatusOK, gin.H{"status": "queued"})

	case "conversation_status_changed":
		// Task 7.1: log status changes for observability; no further action needed.
		log.Printf("[chatwoot][%s] conversation %d status → %s",
			instanceName, payload.Conversation.ID, payload.Conversation.Status)
		c.JSON(http.StatusOK, gin.H{"status": "ok"})

	default:
		c.JSON(http.StatusOK, gin.H{"status": "ignored"})
	}
}

// dispatchToWhatsApp sends the Chatwoot agent message to WhatsApp (Tasks 7.2 / 7.3 / 7.4).
func (ctrl *ChatwootController) dispatchToWhatsApp(instanceName string, payload chatwoot_dto.ChatwootWebhookPayload) {
	// Task 7.2: extract phone number
	phone := payload.Conversation.Meta.Sender.PhoneNumber
	if phone == "" {
		return
	}
	phone = strings.TrimPrefix(phone, "+")

	instance, err := ctrl.instanceRepository.GetInstanceByName(instanceName)
	if err != nil || !instance.Connected {
		return
	}

	// Task 7.4: media attachment — send as media URL
	if len(payload.Attachments) > 0 {
		att := payload.Attachments[0]
		mediaType := resolveMediaType(att.ContentType)
		ctrl.sendService.SendMediaUrl(&send_service.MediaStruct{ //nolint:errcheck
			Number:  phone,
			Url:     att.DataURL,
			Type:    mediaType,
			Caption: payload.Content,
		}, instance)
		return
	}

	// Task 7.3: plain text message
	if payload.Content != "" {
		ctrl.sendService.SendText(&send_service.TextStruct{ //nolint:errcheck
			Number: phone,
			Text:   payload.Content,
		}, instance)
	}
}

// handleInboxWhatsappCommand processes the #inbox_whatsapp:{INSTANCE_NAME} command (Task 7.5).
// It creates a new Chatwoot inbox for the target instance using the current instance's credentials.
func (ctrl *ChatwootController) handleInboxWhatsappCommand(instanceName string, payload chatwoot_dto.ChatwootWebhookPayload) {
	targetInstance := strings.TrimSpace(strings.TrimPrefix(payload.Content, "#inbox_whatsapp:"))
	if targetInstance == "" {
		log.Printf("[chatwoot][%s] #inbox_whatsapp: command missing target instance name", instanceName)
		return
	}

	currentSetting, err := ctrl.chatwootService.FindChatwoot(instanceName)
	if err != nil {
		log.Printf("[chatwoot][%s] #inbox_whatsapp: could not load current config: %v", instanceName, err)
		return
	}

	_, err = ctrl.chatwootService.SetChatwoot(targetInstance, &chatwoot_dto.SetChatwootRequest{
		Enabled:    true,
		AccountID:  currentSetting.AccountID,
		Token:      currentSetting.Token,
		URL:        currentSetting.URL,
		NameInbox:  targetInstance,
		AutoCreate: true,
	})
	if err != nil {
		log.Printf("[chatwoot][%s] #inbox_whatsapp: failed to configure instance %q: %v",
			instanceName, targetInstance, err)
		return
	}

	log.Printf("[chatwoot][%s] #inbox_whatsapp: inbox created for instance %q", instanceName, targetInstance)
}

func resolveMediaType(contentType string) string {
	switch {
	case strings.HasPrefix(contentType, "image/"):
		return "image"
	case strings.HasPrefix(contentType, "video/"):
		return "video"
	case strings.HasPrefix(contentType, "audio/"):
		return "audio"
	default:
		return "document"
	}
}
