package chat

import (
	"encoding/json"
	"fmt"
	"strings"
)

const contentPartTypeText = "text"

type MessageContent struct {
	Parts []MessageContentPart `json:"parts"`
}

type MessageContentPart struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

func NewTextContent(text string) (json.RawMessage, error) {
	content := MessageContent{
		Parts: make([]MessageContentPart, 0, 1),
	}

	if trimmed := strings.TrimSpace(text); trimmed != "" {
		content.Parts = append(content.Parts, MessageContentPart{
			Type: contentPartTypeText,
			Text: trimmed,
		})
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
