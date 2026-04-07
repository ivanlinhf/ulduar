package chat

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/ivanlin/ulduar/apps/backend/internal/azureopenai"
)

const contentPartTypeText = "text"

type MessageContent struct {
	Parts []MessageContentPart `json:"parts"`
}

type MessageContentPart struct {
	Type      string            `json:"type"`
	Text      string            `json:"text,omitempty"`
	Citations []MessageCitation `json:"citations,omitempty"`
}

type MessageCitation struct {
	Title      string `json:"title,omitempty"`
	URL        string `json:"url,omitempty"`
	StartIndex *int   `json:"startIndex,omitempty"`
	EndIndex   *int   `json:"endIndex,omitempty"`
}

func NewTextContent(text string) (json.RawMessage, error) {
	return NewTextContentWithCitations(text, nil)
}

func NewTextContentWithCitations(text string, citations []MessageCitation) (json.RawMessage, error) {
	content := MessageContent{
		Parts: make([]MessageContentPart, 0, 1),
	}

	if trimmed := strings.TrimSpace(text); trimmed != "" {
		storedText := trimmed
		if len(citations) > 0 {
			storedText = text
		}

		part := MessageContentPart{
			Type: contentPartTypeText,
			Text: storedText,
		}
		if len(citations) > 0 {
			part.Citations = citations
		}
		content.Parts = append(content.Parts, part)
	}

	return marshalContent(content)
}

func NewEmptyContent() json.RawMessage {
	content, err := marshalContent(MessageContent{
		Parts: []MessageContentPart{},
	})
	if err != nil {
		panic(err)
	}

	return content
}

func DecodeContent(data json.RawMessage) (MessageContent, error) {
	if len(data) == 0 {
		return MessageContent{Parts: []MessageContentPart{}}, nil
	}

	var content MessageContent
	if err := json.Unmarshal(data, &content); err != nil {
		return MessageContent{}, fmt.Errorf("decode message content: %w", err)
	}

	if content.Parts == nil {
		content.Parts = []MessageContentPart{}
	}

	return content, nil
}

func marshalContent(content MessageContent) (json.RawMessage, error) {
	data, err := json.Marshal(content)
	if err != nil {
		return nil, fmt.Errorf("marshal message content: %w", err)
	}

	return data, nil
}

func NewAssistantContentFromResponse(response azureopenai.Response) (json.RawMessage, error) {
	content := MessageContent{
		Parts: make([]MessageContentPart, 0, len(response.Output)),
	}

	for _, item := range response.Output {
		if !isAssistantOutputItem(item) {
			continue
		}
		for _, part := range item.Content {
			if part.Type != "output_text" && part.Type != "text" {
				continue
			}
			trimmedText := strings.TrimSpace(part.Text)
			if trimmedText == "" {
				continue
			}

			contentPart := MessageContentPart{
				Type: contentPartTypeText,
				Text: trimmedText,
			}
			if citations := citationsFromAnnotations(part.Annotations); len(citations) > 0 {
				contentPart.Text = part.Text
				contentPart.Citations = citations
			}
			content.Parts = append(content.Parts, contentPart)
		}
	}

	if len(content.Parts) == 0 {
		return NewTextContent(strings.TrimSpace(response.OutputText))
	}

	return marshalContent(content)
}

func citationsFromAnnotations(annotations []azureopenai.ResponseAnnotation) []MessageCitation {
	citations := make([]MessageCitation, 0, len(annotations))
	for _, annotation := range annotations {
		if annotation.Type != "url_citation" {
			continue
		}
		citationURL := normalizeCitationURL(annotation.URL)
		if citationURL == "" {
			continue
		}

		citations = append(citations, MessageCitation{
			Title:      strings.TrimSpace(annotation.Title),
			URL:        citationURL,
			StartIndex: annotation.StartIndex,
			EndIndex:   annotation.EndIndex,
		})
	}

	if len(citations) == 0 {
		return nil
	}

	return citations
}

func isAssistantOutputItem(item azureopenai.ResponseItem) bool {
	if item.Type != "" && item.Type != "message" {
		return false
	}
	if item.Role != "" && item.Role != messageRoleAssistant {
		return false
	}

	return true
}

func normalizeCitationURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed == nil {
		return ""
	}
	if parsed.Host == "" {
		return ""
	}

	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		return parsed.String()
	default:
		return ""
	}
}
