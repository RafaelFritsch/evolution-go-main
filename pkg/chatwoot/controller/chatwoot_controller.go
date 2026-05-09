package chatwoot_controller

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	chatwoot_dto "github.com/EvolutionAPI/evolution-go/pkg/chatwoot/dto"
	chatwoot_service "github.com/EvolutionAPI/evolution-go/pkg/chatwoot/service"
	instance_repository "github.com/EvolutionAPI/evolution-go/pkg/instance/repository"
	logger_wrapper "github.com/EvolutionAPI/evolution-go/pkg/logger"
	send_service "github.com/EvolutionAPI/evolution-go/pkg/sendMessage/service"
)

type ChatwootController struct {
	chatwootService    chatwoot_service.ChatwootService
	sendService        send_service.SendService
	instanceRepository instance_repository.InstanceRepository
	loggerManager      *logger_wrapper.LoggerManager
}

func NewController(
	svc chatwoot_service.ChatwootService,
	sendSvc send_service.SendService,
	instanceRepo instance_repository.InstanceRepository,
	loggerManager *logger_wrapper.LoggerManager,
) *ChatwootController {
	return &ChatwootController{
		chatwootService:    svc,
		sendService:        sendSvc,
		instanceRepository: instanceRepo,
		loggerManager:      loggerManager,
	}
}

func (ctrl *ChatwootController) log(instanceName string) *logger_wrapper.Logger {
	return ctrl.loggerManager.GetLogger(instanceName)
}

// SetChatwoot godoc
// @Summary      Configure Chatwoot integration for an instance
// @Description  Creates or updates the Chatwoot settings associated with the provided instance.
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
		"id":         setting.ID,
		"instanceId": setting.InstanceID,
		"enabled":    setting.Enabled,
		"accountId":  setting.AccountID,
		"token":      setting.Token,
		"url":        setting.URL,
		"nameInbox":  setting.NameInbox,
		"inboxId":    setting.InboxID,
		"ignoreJids": parseIgnoreJIDs(setting.IgnoreJids),
	})
}

// FindChatwoot godoc
// @Summary      Get Chatwoot configuration for an instance
// @Description  Returns the persisted Chatwoot settings for the provided instance.
// @Tags         Chatwoot
// @Produce      json
// @Param        instance  path      string  true  "Instance name"
// @Success      200       {object}  chatwoot_dto.ChatwootResponse
// @Failure      404       {object}  gin.H
// @Failure      500       {object}  gin.H
// @Router       /chatwoot/find/{instance} [get]
func (ctrl *ChatwootController) FindChatwoot(c *gin.Context) {
	instanceName := c.Param("instance")

	setting, err := ctrl.chatwootService.FindChatwoot(instanceName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":                      setting.ID,
		"instanceId":              setting.InstanceID,
		"enabled":                 setting.Enabled,
		"accountId":               setting.AccountID,
		"token":                   setting.Token,
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
		"ignoreJids":              parseIgnoreJIDs(setting.IgnoreJids),
	})
}

// ReceiveWebhook handles incoming Chatwoot webhook events and dispatches them to WhatsApp.
//
// @Summary      Receive webhook from Chatwoot
// @Description  Receives Chatwoot webhook events for a mapped instance and forwards supported outgoing messages to WhatsApp.
// @Tags         Chatwoot
// @Accept       json
// @Produce      json
// @Param        instance  path      string                               true  "Instance name"
// @Param        body      body      chatwoot_dto.ChatwootWebhookPayload  true  "Webhook payload"
// @Success      200       {object}  gin.H
// @Failure      400       {object}  gin.H
// @Router       /chatwoot/webhook/{instance} [post]
func (ctrl *ChatwootController) ReceiveWebhook(c *gin.Context) {
	instanceName := c.Param("instance")

	var payload chatwoot_dto.ChatwootWebhookPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		ctrl.log(instanceName).LogWarnWithMetadata(
			"[chatwoot] webhook ignored due to invalid payload",
			map[string]interface{}{
				"instance": instanceName,
				"error":    err.Error(),
			},
		)
		c.JSON(http.StatusOK, gin.H{"status": "ignored"})
		return
	}

	// Task 7.1: route by event type
	switch payload.Event {
	case "message_created":
		ctrl.log(instanceName).LogInfoWithMetadata(
			"[chatwoot] webhook received",
			map[string]interface{}{
				"instance":        instanceName,
				"event":           payload.Event,
				"message_type":    payload.MessageType,
				"private":         payload.Private,
				"conversation_id": payload.Conversation.ID,
				"attachments":     len(payload.Attachments),
			},
		)
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
		ctrl.log(instanceName).LogInfoWithMetadata(
			"[chatwoot] conversation status changed",
			map[string]interface{}{
				"instance":            instanceName,
				"event":               payload.Event,
				"conversation_id":     payload.Conversation.ID,
				"conversation_status": payload.Conversation.Status,
			},
		)
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
		ctrl.log(instanceName).LogWarnWithMetadata(
			"[chatwoot] webhook message ignored because sender phone is empty",
			map[string]interface{}{
				"instance":        instanceName,
				"conversation_id": payload.Conversation.ID,
				"event":           payload.Event,
			},
		)
		return
	}
	phone = strings.TrimPrefix(phone, "+")

	instance, err := ctrl.instanceRepository.GetInstanceByName(instanceName)
	if err != nil || !instance.Connected {
		ctrl.log(instanceName).LogWarnWithMetadata(
			"[chatwoot] webhook message ignored because instance is unavailable",
			map[string]interface{}{
				"instance":        instanceName,
				"conversation_id": payload.Conversation.ID,
				"phone":           phone,
				"connected":       err == nil && instance.Connected,
			},
		)
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
		ctrl.log(instanceName).LogInfoWithMetadata(
			"[chatwoot] webhook media dispatched to whatsapp",
			map[string]interface{}{
				"instance":        instanceName,
				"conversation_id": payload.Conversation.ID,
				"phone":           phone,
				"media_type":      mediaType,
			},
		)
		return
	}

	// Task 7.3: plain text message
	if payload.Content != "" {
		ctrl.sendService.SendText(&send_service.TextStruct{ //nolint:errcheck
			Number: phone,
			Text:   payload.Content,
		}, instance)
		ctrl.log(instanceName).LogInfoWithMetadata(
			"[chatwoot] webhook text dispatched to whatsapp",
			map[string]interface{}{
				"instance":        instanceName,
				"conversation_id": payload.Conversation.ID,
				"phone":           phone,
			},
		)
	}
}

// handleInboxWhatsappCommand processes the #inbox_whatsapp:{INSTANCE_NAME} command (Task 7.5).
// It creates a new Chatwoot inbox for the target instance using the current instance's credentials.
func (ctrl *ChatwootController) handleInboxWhatsappCommand(instanceName string, payload chatwoot_dto.ChatwootWebhookPayload) {
	targetInstance := strings.TrimSpace(strings.TrimPrefix(payload.Content, "#inbox_whatsapp:"))
	if targetInstance == "" {
		ctrl.log(instanceName).LogWarnWithMetadata(
			"[chatwoot] inbox_whatsapp command missing target instance",
			map[string]interface{}{
				"instance": instanceName,
				"content":  payload.Content,
			},
		)
		return
	}

	currentSetting, err := ctrl.chatwootService.FindChatwoot(instanceName)
	if err != nil {
		ctrl.log(instanceName).LogErrorWithMetadata(
			"[chatwoot] inbox_whatsapp command failed to load current config",
			map[string]interface{}{
				"instance":        instanceName,
				"target_instance": targetInstance,
				"error":           err.Error(),
			},
		)
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
		ctrl.log(instanceName).LogErrorWithMetadata(
			"[chatwoot] inbox_whatsapp command failed to configure target instance",
			map[string]interface{}{
				"instance":        instanceName,
				"target_instance": targetInstance,
				"error":           err.Error(),
			},
		)
		return
	}

	ctrl.log(instanceName).LogInfoWithMetadata(
		"[chatwoot] inbox_whatsapp command configured target instance",
		map[string]interface{}{
			"instance":        instanceName,
			"target_instance": targetInstance,
		},
	)
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

func parseIgnoreJIDs(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}

	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return []string{}
	}
	return values
}
