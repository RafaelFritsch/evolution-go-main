package chatwoot_service

import (
	"strings"

	"go.mau.fi/whatsmeow/proto/waE2E"
)

// ExtractMessageContent extracts human-readable text from a whatsmeow message proto.
// Returns (text, mediaType) where mediaType is empty for plain text messages.
func ExtractMessageContent(msg *waE2E.Message) (text string, mediaType string) {
	if msg == nil {
		return "", ""
	}

	if t := msg.GetConversation(); t != "" {
		return t, ""
	}
	if ext := msg.GetExtendedTextMessage(); ext != nil {
		return ext.GetText(), ""
	}
	if img := msg.GetImageMessage(); img != nil {
		return img.GetCaption(), "image"
	}
	if vid := msg.GetVideoMessage(); vid != nil {
		return vid.GetCaption(), "video"
	}
	if doc := msg.GetDocumentMessage(); doc != nil {
		title := doc.GetTitle()
		if title == "" {
			title = doc.GetFileName()
		}
		return title, "document"
	}
	if msg.GetAudioMessage() != nil {
		return "", "audio"
	}
	if msg.GetStickerMessage() != nil {
		return "", "sticker"
	}
	if loc := msg.GetLocationMessage(); loc != nil {
		return formatLocation(loc.GetDegreesLatitude(), loc.GetDegreesLongitude()), "location"
	}
	if msg.GetContactMessage() != nil {
		return "", "contact"
	}
	if poll := msg.GetPollCreationMessage(); poll != nil {
		return poll.GetName(), "poll"
	}
	if react := msg.GetReactionMessage(); react != nil {
		return react.GetText(), "reaction"
	}

	return "", ""
}

// ExtractAudioTranscript returns transcription text for audio messages when the
// upstream event payload already contains it. The project currently does not
// generate transcripts itself, so this is a best-effort extractor.
func ExtractAudioTranscript(data map[string]interface{}, msg *waE2E.Message) string {
	if msg == nil || msg.GetAudioMessage() == nil || data == nil {
		return ""
	}

	if transcript := firstNonEmptyString(
		data["audioTranscription"],
		data["audioTranscript"],
		data["transcription"],
		data["transcript"],
	); transcript != "" {
		return transcript
	}

	messageMap, ok := data["Message"].(map[string]interface{})
	if !ok {
		return ""
	}

	if transcript := firstNonEmptyString(
		messageMap["audioTranscription"],
		messageMap["audioTranscript"],
		messageMap["transcription"],
		messageMap["transcript"],
		messageMap["text"],
	); transcript != "" {
		return transcript
	}

	audioMap, ok := messageMap["audioMessage"].(map[string]interface{})
	if !ok {
		return ""
	}

	return firstNonEmptyString(
		audioMap["audioTranscription"],
		audioMap["audioTranscript"],
		audioMap["transcription"],
		audioMap["transcript"],
		audioMap["text"],
		audioMap["caption"],
	)
}

func formatLocation(lat, lon float64) string {
	var sb strings.Builder
	sb.WriteString("Location: ")
	appendFloat(&sb, lat)
	sb.WriteString(", ")
	appendFloat(&sb, lon)
	return sb.String()
}

func appendFloat(sb *strings.Builder, f float64) {
	// Simple float formatting without importing fmt to keep this helper lean.
	if f < 0 {
		sb.WriteByte('-')
		f = -f
	}
	intPart := int64(f)
	frac := int64((f - float64(intPart)) * 1_000_000)
	writeInt64(sb, intPart)
	sb.WriteByte('.')
	writeInt64Padded(sb, frac, 6)
}

func firstNonEmptyString(values ...interface{}) string {
	for _, value := range values {
		if str, ok := value.(string); ok {
			str = strings.TrimSpace(str)
			if str != "" {
				return str
			}
		}
	}
	return ""
}

func writeInt64(sb *strings.Builder, n int64) {
	if n == 0 {
		sb.WriteByte('0')
		return
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	sb.Write(buf[i:])
}

func writeInt64Padded(sb *strings.Builder, n int64, width int) {
	var buf [20]byte
	i := len(buf)
	for w := 0; w < width; w++ {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	sb.Write(buf[i:])
}
