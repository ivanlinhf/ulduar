package chat

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/ivanlin/ulduar/apps/backend/internal/azureopenai"
	"github.com/ivanlin/ulduar/apps/backend/internal/repository"
)

const (
	providerMessageType      = "message"
	providerInputTextType    = "input_text"
	providerOutputTextType   = "output_text"
	providerInputImageType   = "input_image"
	providerInputFileType    = "input_file"
	providerImageDetailLevel = "auto"
	pdfMediaType             = "application/pdf"
)

func (s *Service) ReconstructConversation(ctx context.Context, sessionID string) ([]azureopenai.InputMessage, error) {
	view, err := s.loadSessionView(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	input, err := s.prepareProviderInput(ctx, view.Messages)
	if err != nil {
		return nil, fmt.Errorf("prepare provider input: %w", err)
	}

	return input, nil
}

func (s *Service) ReconstructConversationForTurn(ctx context.Context, sessionID, userMessageID string) ([]azureopenai.InputMessage, error) {
	view, err := s.loadSessionView(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	messages, err := trimMessagesThroughID(view.Messages, userMessageID)
	if err != nil {
		return nil, err
	}

	input, err := s.prepareProviderInput(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("prepare provider input: %w", err)
	}

	return input, nil
}

func (s *Service) prepareProviderInput(ctx context.Context, messages []MessageView) ([]azureopenai.InputMessage, error) {
	input := make([]azureopenai.InputMessage, 0, len(messages))
	for _, message := range messages {
		if !shouldIncludeProviderMessage(message.Message) {
			continue
		}

		item, err := s.prepareProviderMessage(ctx, message)
		if err != nil {
			return nil, fmt.Errorf("message %s: %w", message.Message.ID, err)
		}
		if len(item.Content) == 0 {
			continue
		}

		input = append(input, item)
	}

	return input, nil
}

func (s *Service) prepareProviderMessage(ctx context.Context, message MessageView) (azureopenai.InputMessage, error) {
	content := make([]azureopenai.InputContentItem, 0, len(message.Content.Parts)+len(message.Attachments))
	textContentType := providerTextContentType(message.Message.Role)

	for _, part := range message.Content.Parts {
		switch part.Type {
		case contentPartTypeText:
			if text := strings.TrimSpace(part.Text); text != "" {
				content = append(content, azureopenai.InputContentItem{
					Type: textContentType,
					Text: text,
				})
			}
		default:
			return azureopenai.InputMessage{}, fmt.Errorf("unsupported message content part type %q", part.Type)
		}
	}

	for _, attachment := range message.Attachments {
		item, err := s.prepareAttachmentInput(ctx, attachment)
		if err != nil {
			return azureopenai.InputMessage{}, err
		}
		content = append(content, item)
	}

	return azureopenai.InputMessage{
		Type:    providerMessageType,
		Role:    message.Message.Role,
		Content: content,
	}, nil
}

func providerTextContentType(role string) string {
	if role == messageRoleAssistant {
		return providerOutputTextType
	}

	return providerInputTextType
}

func (s *Service) prepareAttachmentInput(ctx context.Context, attachment repository.Attachment) (azureopenai.InputContentItem, error) {
	data, err := s.blobs.Download(ctx, attachment.BlobPath)
	if err != nil {
		return azureopenai.InputContentItem{}, fmt.Errorf("load attachment %s blob: %w", attachment.ID, err)
	}

	switch {
	case strings.HasPrefix(attachment.MediaType, "image/"):
		return azureopenai.InputContentItem{
			Type:     providerInputImageType,
			ImageURL: buildDataURL(attachment.MediaType, data),
			Detail:   providerImageDetailLevel,
		}, nil
	case attachment.MediaType == pdfMediaType:
		return azureopenai.InputContentItem{
			Type:     providerInputFileType,
			FileData: base64.StdEncoding.EncodeToString(data),
			Filename: attachment.Filename,
		}, nil
	default:
		return azureopenai.InputContentItem{}, fmt.Errorf("unsupported attachment media type %q", attachment.MediaType)
	}
}

func shouldIncludeProviderMessage(message repository.Message) bool {
	return message.Status == messageStatusComplete
}

func trimMessagesThroughID(messages []MessageView, messageID string) ([]MessageView, error) {
	trimmed := make([]MessageView, 0, len(messages))
	for _, message := range messages {
		trimmed = append(trimmed, message)
		if message.Message.ID == messageID {
			return trimmed, nil
		}
	}

	return nil, fmt.Errorf("message %s not found in reconstructed history", messageID)
}

func buildDataURL(mediaType string, data []byte) string {
	return fmt.Sprintf(
		"data:%s;base64,%s",
		mediaType,
		base64.StdEncoding.EncodeToString(data),
	)
}
