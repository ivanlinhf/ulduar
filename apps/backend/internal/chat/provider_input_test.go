package chat

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/ivanlin/ulduar/apps/backend/internal/azureopenai"
	"github.com/ivanlin/ulduar/apps/backend/internal/repository"
)

type stubBlobStore struct {
	data map[string][]byte
}

func (s stubBlobStore) Upload(ctx context.Context, blobPath string, data []byte, contentType string) error {
	return nil
}

func (s stubBlobStore) Delete(ctx context.Context, blobPath string) error {
	return nil
}

func (s stubBlobStore) Download(ctx context.Context, blobPath string) ([]byte, error) {
	return s.data[blobPath], nil
}

func TestPrepareProviderInputBuildsTextImageAndPDFMessages(t *testing.T) {
	t.Parallel()

	const (
		imageBlobPath = "sessions/session/messages/user/attachments/01-image"
		pdfBlobPath   = "sessions/session/messages/user/attachments/02-pdf"
	)

	imageData := []byte("png-bytes")
	pdfData := []byte("%PDF-1.7")

	userContent, err := NewTextContent("Describe these files")
	if err != nil {
		t.Fatalf("NewTextContent() error = %v", err)
	}

	assistantContent, err := NewTextContent("The files look valid.")
	if err != nil {
		t.Fatalf("NewTextContent() error = %v", err)
	}

	decodedUserContent, err := DecodeContent(userContent)
	if err != nil {
		t.Fatalf("DecodeContent(user) error = %v", err)
	}

	decodedAssistantContent, err := DecodeContent(assistantContent)
	if err != nil {
		t.Fatalf("DecodeContent(assistant) error = %v", err)
	}

	service := NewService(nil, stubBlobStore{
		data: map[string][]byte{
			imageBlobPath: imageData,
			pdfBlobPath:   pdfData,
		},
	}, nil, ServiceOptions{})

	input, err := service.prepareProviderInput(context.Background(), []MessageView{
		{
			Message: repository.Message{
				ID:     "user-message",
				Role:   messageRoleUser,
				Status: messageStatusComplete,
			},
			Content: decodedUserContent,
			Attachments: []repository.Attachment{
				{
					ID:        "attachment-image",
					BlobPath:  imageBlobPath,
					MediaType: "image/png",
					Filename:  "photo.png",
				},
				{
					ID:        "attachment-pdf",
					BlobPath:  pdfBlobPath,
					MediaType: pdfMediaType,
					Filename:  "notes.pdf",
				},
			},
		},
		{
			Message: repository.Message{
				ID:     "assistant-message",
				Role:   messageRoleAssistant,
				Status: messageStatusComplete,
			},
			Content: decodedAssistantContent,
		},
		{
			Message: repository.Message{
				ID:     "pending-assistant",
				Role:   messageRoleAssistant,
				Status: messageStatusPending,
			},
			Content: MessageContent{Parts: []MessageContentPart{}},
		},
	})
	if err != nil {
		t.Fatalf("prepareProviderInput() error = %v", err)
	}

	if len(input) != 2 {
		t.Fatalf("expected 2 provider messages, got %d", len(input))
	}

	userMessage := input[0]
	if userMessage.Role != messageRoleUser {
		t.Fatalf("expected first provider role %q, got %q", messageRoleUser, userMessage.Role)
	}
	if len(userMessage.Content) != 3 {
		t.Fatalf("expected 3 user content items, got %d", len(userMessage.Content))
	}

	if userMessage.Content[0].Type != providerInputTextType || userMessage.Content[0].Text != "Describe these files" {
		t.Fatalf("unexpected text content: %#v", userMessage.Content[0])
	}

	expectedImageURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(imageData)
	if userMessage.Content[1].Type != providerInputImageType || userMessage.Content[1].ImageURL != expectedImageURL {
		t.Fatalf("unexpected image content: %#v", userMessage.Content[1])
	}
	if userMessage.Content[1].Detail != providerImageDetailLevel {
		t.Fatalf("expected image detail %q, got %q", providerImageDetailLevel, userMessage.Content[1].Detail)
	}

	expectedPDF := base64.StdEncoding.EncodeToString(pdfData)
	if userMessage.Content[2].Type != providerInputFileType || userMessage.Content[2].FileData != expectedPDF {
		t.Fatalf("unexpected file content: %#v", userMessage.Content[2])
	}
	if userMessage.Content[2].Filename != "notes.pdf" {
		t.Fatalf("expected pdf filename notes.pdf, got %q", userMessage.Content[2].Filename)
	}

	assistantMessage := input[1]
	if assistantMessage.Role != messageRoleAssistant {
		t.Fatalf("expected assistant role %q, got %q", messageRoleAssistant, assistantMessage.Role)
	}
	if len(assistantMessage.Content) != 1 || assistantMessage.Content[0].Text != "The files look valid." {
		t.Fatalf("unexpected assistant content: %#v", assistantMessage.Content)
	}
}

func TestPrepareProviderInputRejectsUnsupportedAttachmentType(t *testing.T) {
	t.Parallel()

	service := NewService(nil, stubBlobStore{
		data: map[string][]byte{
			"blob-path": []byte("csv-data"),
		},
	}, nil, ServiceOptions{})

	input, err := service.prepareProviderInput(context.Background(), []MessageView{
		{
			Message: repository.Message{
				ID:     "user-message",
				Role:   messageRoleUser,
				Status: messageStatusComplete,
			},
			Content: MessageContent{Parts: []MessageContentPart{}},
			Attachments: []repository.Attachment{
				{
					ID:        "attachment-csv",
					BlobPath:  "blob-path",
					MediaType: "text/csv",
					Filename:  "bad.csv",
				},
			},
		},
	})
	if err == nil {
		t.Fatalf("expected error, got input %#v", input)
	}
	if !strings.Contains(err.Error(), `unsupported attachment media type "text/csv"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTrimMessagesThroughIDStopsAtRequestedTurn(t *testing.T) {
	t.Parallel()

	messages := []MessageView{
		{Message: repository.Message{ID: "m1", Role: messageRoleUser}},
		{Message: repository.Message{ID: "m2", Role: messageRoleAssistant}},
		{Message: repository.Message{ID: "m3", Role: messageRoleUser}},
	}

	trimmed, err := trimMessagesThroughID(messages, "m2")
	if err != nil {
		t.Fatalf("trimMessagesThroughID() error = %v", err)
	}

	if len(trimmed) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(trimmed))
	}
	if trimmed[1].Message.ID != "m2" {
		t.Fatalf("expected last trimmed message m2, got %q", trimmed[1].Message.ID)
	}
}

func TestExtractResponseTextFallsBackToOutputItems(t *testing.T) {
	t.Parallel()

	text := extractResponseText(azureopenai.Response{
		Output: []azureopenai.ResponseItem{
			{
				Content: []azureopenai.ResponseContentItem{
					{Type: "output_text", Text: "First part"},
					{Type: "text", Text: "Second part"},
				},
			},
		},
	})

	if text != "First part\n\nSecond part" {
		t.Fatalf("unexpected extracted text: %q", text)
	}
}

func TestContentTextJoinsTextParts(t *testing.T) {
	t.Parallel()

	text := contentText(MessageContent{
		Parts: []MessageContentPart{
			{Type: contentPartTypeText, Text: "One"},
			{Type: "ignored", Text: "nope"},
			{Type: contentPartTypeText, Text: "Two"},
		},
	})

	if text != "One\n\nTwo" {
		t.Fatalf("unexpected content text: %q", text)
	}
}
