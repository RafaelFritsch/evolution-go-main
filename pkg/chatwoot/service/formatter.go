package chatwoot_service

import "strings"

// FormatWhatsAppToMarkdown converts WhatsApp text formatting to standard Markdown.
//
// WhatsApp  → Markdown
//   *bold*  → **bold**
//   ~strike~ → ~~strike~~
//   _italic_ stays _italic_ (already compatible)
func FormatWhatsAppToMarkdown(text string) string {
	if text == "" {
		return text
	}

	var out strings.Builder
	out.Grow(len(text) + 16)

	i := 0
	for i < len(text) {
		ch := text[i]

		switch ch {
		case '*':
			// WhatsApp bold *text* → Markdown **text**
			if end := findClosing(text, i+1, '*'); end > 0 {
				out.WriteString("**")
				out.WriteString(text[i+1 : end])
				out.WriteString("**")
				i = end + 1
				continue
			}
		case '~':
			// WhatsApp strikethrough ~text~ → Markdown ~~text~~
			if end := findClosing(text, i+1, '~'); end > 0 {
				out.WriteString("~~")
				out.WriteString(text[i+1 : end])
				out.WriteString("~~")
				i = end + 1
				continue
			}
		}

		out.WriteByte(ch)
		i++
	}

	return out.String()
}

// findClosing returns the index of the next unescaped occurrence of delim
// after position start, or -1 if none found before end-of-string or newline.
func findClosing(s string, start int, delim byte) int {
	for j := start; j < len(s); j++ {
		if s[j] == '\n' {
			return -1
		}
		if s[j] == delim {
			return j
		}
	}
	return -1
}
