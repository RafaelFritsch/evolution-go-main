package chatwoot_dto

type SetChatwootRequest struct {
	Enabled                 bool     `json:"enabled"`
	AccountID               string   `json:"accountId" binding:"required"`
	Token                   string   `json:"token" binding:"required"`
	URL                     string   `json:"url" binding:"required"`
	SignMsg                 bool     `json:"signMsg"`
	SignDelimiter           string   `json:"signDelimiter"`
	ReopenConversation      bool     `json:"reopenConversation"`
	ConversationPending     bool     `json:"conversationPending"`
	NameInbox               string   `json:"nameInbox"`
	MergeBrazilContacts     bool     `json:"mergeBrazilContacts"`
	ImportContacts          bool     `json:"importContacts"`
	ImportMessages          bool     `json:"importMessages"`
	DaysLimitImportMessages int      `json:"daysLimitImportMessages"`
	AutoCreate              bool     `json:"autoCreate"`
	Organization            string   `json:"organization"`
	Logo                    string   `json:"logo"`
	IgnoreJids              []string `json:"ignoreJids"`
}

type ChatwootResponse struct {
	ID                      uint     `json:"id"`
	InstanceID              string   `json:"instanceId"`
	Enabled                 bool     `json:"enabled"`
	AccountID               string   `json:"accountId"`
	Token                   string   `json:"token"`
	URL                     string   `json:"url"`
	SignMsg                 bool     `json:"signMsg"`
	SignDelimiter           string   `json:"signDelimiter"`
	ReopenConversation      bool     `json:"reopenConversation"`
	ConversationPending     bool     `json:"conversationPending"`
	NameInbox               string   `json:"nameInbox"`
	InboxID                 int64    `json:"inboxId"`
	MergeBrazilContacts     bool     `json:"mergeBrazilContacts"`
	ImportContacts          bool     `json:"importContacts"`
	ImportMessages          bool     `json:"importMessages"`
	DaysLimitImportMessages int      `json:"daysLimitImportMessages"`
	AutoCreate              bool     `json:"autoCreate"`
	Organization            string   `json:"organization"`
	Logo                    string   `json:"logo"`
	IgnoreJids              []string `json:"ignoreJids"`
}

type ChatwootWebhookContact struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	PhoneNumber string `json:"phone_number"`
	Identifier  string `json:"identifier"`
}

type ChatwootWebhookConversationMeta struct {
	Sender   ChatwootWebhookContact `json:"sender"`
	Receiver ChatwootWebhookContact `json:"receiver"`
	Channel  string                 `json:"channel"`
}

type ChatwootWebhookAttachment struct {
	ID          int64  `json:"id"`
	DataURL     string `json:"data_url"`
	FileType    string `json:"file_type"`
	ContentType string `json:"content_type"`
}

type ChatwootWebhookConversation struct {
	ID     int64                           `json:"id"`
	Status string                          `json:"status"`
	Meta   ChatwootWebhookConversationMeta `json:"meta"`
}

type ChatwootWebhookPayload struct {
	ID           int64                       `json:"id"`
	Event        string                      `json:"event"`
	MessageType  string                      `json:"message_type"`
	Content      string                      `json:"content"`
	ContentType  string                      `json:"content_type"`
	Private      bool                        `json:"private"`
	Conversation ChatwootWebhookConversation `json:"conversation"`
	Contact      ChatwootWebhookContact      `json:"contact"`
	Attachments  []ChatwootWebhookAttachment `json:"attachments"`
}
