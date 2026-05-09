package chatwoot_client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type ChatwootClient struct {
	BaseURL    string
	Token      string
	AccountID  string
	HTTPClient *http.Client
}

func NewChatwootClient(baseURL, token, accountID string) *ChatwootClient {
	return &ChatwootClient{
		BaseURL:   baseURL,
		Token:     token,
		AccountID: accountID,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type Contact struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	PhoneNumber string `json:"phone_number"`
	Identifier  string `json:"identifier"`
	AvatarURL   string `json:"avatar_url,omitempty"`
}

type CreateContactPayload struct {
	Name        string `json:"name,omitempty"`
	PhoneNumber string `json:"phone_number,omitempty"`
	Identifier  string `json:"identifier,omitempty"`
	AvatarURL   string `json:"avatar_url,omitempty"`
}

type contactSearchResponse struct {
	Payload struct {
		Contacts []Contact `json:"contacts"`
	} `json:"payload"`
}

type Inbox struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type inboxListResponse struct {
	Payload []Inbox `json:"payload"`
}

type createInboxPayload struct {
	Name    string `json:"name"`
	Channel struct {
		Type       string `json:"type"`
		WebhookURL string `json:"webhook_url"`
	} `json:"channel"`
}

type Conversation struct {
	ID      int64  `json:"id"`
	Status  string `json:"status"`
	InboxID int64  `json:"inbox_id"`
}

type conversationListResponse struct {
	Payload []Conversation `json:"payload"`
}

type CreateConversationPayload struct {
	ContactID            int64             `json:"contact_id"`
	InboxID              int64             `json:"inbox_id"`
	Status               string            `json:"status"`
	AdditionalAttributes map[string]string `json:"additional_attributes,omitempty"`
}

type toggleStatusPayload struct {
	Status string `json:"status"`
}

type MessageAttachment struct {
	ResourceType string `json:"resource_type"`
	ResourceURL  string `json:"resource_url"`
}

type CreateMessagePayload struct {
	Content     string              `json:"content"`
	MessageType string              `json:"message_type"`
	Private     bool                `json:"private"`
	ContentType string              `json:"content_type"`
	Attachments []MessageAttachment `json:"attachments,omitempty"`
	CreatedAt   *int64              `json:"created_at,omitempty"`
}

type Message struct {
	ID             int64  `json:"id"`
	ConversationID int64  `json:"conversation_id"`
	Content        string `json:"content"`
	MessageType    string `json:"message_type"`
}

func (c *ChatwootClient) do(method, path string, body interface{}) ([]byte, int, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.BaseURL+path, reqBody)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api_access_token", c.Token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	if resp.StatusCode >= 400 {
		return nil, resp.StatusCode, fmt.Errorf("chatwoot API %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, resp.StatusCode, nil
}

func (c *ChatwootClient) CreateContact(payload CreateContactPayload) (*Contact, error) {
	path := fmt.Sprintf("/api/v1/accounts/%s/contacts", c.AccountID)
	data, _, err := c.do("POST", path, payload)
	if err != nil {
		return nil, err
	}
	var contact Contact
	return &contact, json.Unmarshal(data, &contact)
}

func (c *ChatwootClient) FindContact(query string) ([]Contact, error) {
	path := fmt.Sprintf("/api/v1/accounts/%s/contacts/search?q=%s&include_contacts=true",
		c.AccountID, url.QueryEscape(query))
	data, _, err := c.do("GET", path, nil)
	if err != nil {
		return nil, err
	}
	var resp contactSearchResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return resp.Payload.Contacts, nil
}

func (c *ChatwootClient) UpdateContact(contactID int64, payload CreateContactPayload) (*Contact, error) {
	path := fmt.Sprintf("/api/v1/accounts/%s/contacts/%d", c.AccountID, contactID)
	data, _, err := c.do("PATCH", path, payload)
	if err != nil {
		return nil, err
	}
	var contact Contact
	return &contact, json.Unmarshal(data, &contact)
}

func (c *ChatwootClient) MergeContacts(parentID, childID int64) error {
	path := fmt.Sprintf("/api/v1/accounts/%s/contacts/%d/merge", c.AccountID, parentID)
	_, _, err := c.do("POST", path, map[string]int64{"child_id": childID})
	return err
}

func (c *ChatwootClient) GetConversations(contactID, inboxID int64) ([]Conversation, error) {
	path := fmt.Sprintf("/api/v1/accounts/%s/contacts/%d/conversations", c.AccountID, contactID)
	data, _, err := c.do("GET", path, nil)
	if err != nil {
		return nil, err
	}
	var resp conversationListResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	if inboxID == 0 {
		return resp.Payload, nil
	}
	var filtered []Conversation
	for _, conv := range resp.Payload {
		if conv.InboxID == inboxID {
			filtered = append(filtered, conv)
		}
	}
	return filtered, nil
}

func (c *ChatwootClient) CreateConversation(payload CreateConversationPayload) (*Conversation, error) {
	path := fmt.Sprintf("/api/v1/accounts/%s/conversations", c.AccountID)
	data, _, err := c.do("POST", path, payload)
	if err != nil {
		return nil, err
	}
	var conv Conversation
	return &conv, json.Unmarshal(data, &conv)
}

func (c *ChatwootClient) ToggleConversationStatus(conversationID int64, status string) error {
	path := fmt.Sprintf("/api/v1/accounts/%s/conversations/%d/toggle_status", c.AccountID, conversationID)
	_, _, err := c.do("POST", path, toggleStatusPayload{Status: status})
	return err
}

func (c *ChatwootClient) CreateMessage(conversationID int64, payload CreateMessagePayload) (*Message, error) {
	path := fmt.Sprintf("/api/v1/accounts/%s/conversations/%d/messages", c.AccountID, conversationID)
	data, _, err := c.do("POST", path, payload)
	if err != nil {
		return nil, err
	}
	var msg Message
	return &msg, json.Unmarshal(data, &msg)
}

func (c *ChatwootClient) DeleteMessage(conversationID, messageID int64) error {
	path := fmt.Sprintf("/api/v1/accounts/%s/conversations/%d/messages/%d", c.AccountID, conversationID, messageID)
	_, _, err := c.do("DELETE", path, nil)
	return err
}

func (c *ChatwootClient) CreateInbox(name, webhookURL string) (*Inbox, error) {
	path := fmt.Sprintf("/api/v1/accounts/%s/inboxes", c.AccountID)
	payload := createInboxPayload{Name: name}
	payload.Channel.Type = "api"
	payload.Channel.WebhookURL = webhookURL
	data, _, err := c.do("POST", path, payload)
	if err != nil {
		return nil, err
	}
	var inbox Inbox
	return &inbox, json.Unmarshal(data, &inbox)
}

func (c *ChatwootClient) GetInboxes() ([]Inbox, error) {
	path := fmt.Sprintf("/api/v1/accounts/%s/inboxes", c.AccountID)
	data, _, err := c.do("GET", path, nil)
	if err != nil {
		return nil, err
	}
	var resp inboxListResponse
	return resp.Payload, json.Unmarshal(data, &resp)
}
