package chat

import (
	"context"
	"testing"

	"github.com/ivanlin/ulduar/apps/backend/internal/azureopenai"
	"github.com/ivanlin/ulduar/apps/backend/internal/repository"
)

type stubResponseClient struct{}

func (stubResponseClient) CreateResponse(context.Context, azureopenai.CreateResponseRequest) (azureopenai.Response, error) {
	return azureopenai.Response{}, nil
}

func (stubResponseClient) StreamResponse(context.Context, azureopenai.CreateResponseRequest, func(azureopenai.StreamEvent) error) error {
	return nil
}

func TestStreamRunReplaysCompletedRunWithPersistedTokenUsage(t *testing.T) {
	t.Parallel()

	const (
		sessionID          = "11111111-1111-1111-1111-111111111111"
		runID              = "22222222-2222-2222-2222-222222222222"
		assistantMessageID = "33333333-3333-3333-3333-333333333333"
		providerResponseID = "resp_123"
		modelName          = "gpt-5-chat"
		assistantText      = "Token usage badge rendered."
	)

	inputTokens := int64(45)
	outputTokens := int64(78)
	totalTokens := int64(123)

	content, err := NewTextContent(assistantText)
	if err != nil {
		t.Fatalf("NewTextContent() error = %v", err)
	}

	service := NewService(nil, nil, stubResponseClient{}, ServiceOptions{})
	service.loadRunForSessionFn = func(ctx context.Context, receivedSessionID, receivedRunID string) (repository.Run, error) {
		if receivedSessionID != sessionID {
			t.Fatalf("sessionID = %q, want %q", receivedSessionID, sessionID)
		}
		if receivedRunID != runID {
			t.Fatalf("runID = %q, want %q", receivedRunID, runID)
		}

		return repository.Run{
			ID:                 runID,
			SessionID:          sessionID,
			AssistantMessageID: assistantMessageID,
			ProviderResponseID: providerResponseID,
			InputTokens:        &inputTokens,
			OutputTokens:       &outputTokens,
			TotalTokens:        &totalTokens,
			Status:             runStatusCompleted,
		}, nil
	}
	service.getMessageByIDFn = func(ctx context.Context, messageID string) (repository.Message, error) {
		if messageID != assistantMessageID {
			t.Fatalf("messageID = %q, want %q", messageID, assistantMessageID)
		}

		return repository.Message{
			ID:        assistantMessageID,
			Content:   content,
			ModelName: modelName,
		}, nil
	}

	var events []RunStreamEvent
	err = service.StreamRun(context.Background(), sessionID, runID, func(event RunStreamEvent) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatalf("StreamRun() error = %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	if events[0].Type != "message.delta" {
		t.Fatalf("events[0].Type = %q, want %q", events[0].Type, "message.delta")
	}
	if events[0].RunID != runID {
		t.Fatalf("events[0].RunID = %q, want %q", events[0].RunID, runID)
	}
	if events[0].MessageID != assistantMessageID {
		t.Fatalf("events[0].MessageID = %q, want %q", events[0].MessageID, assistantMessageID)
	}
	if events[0].Delta != assistantText {
		t.Fatalf("events[0].Delta = %q, want %q", events[0].Delta, assistantText)
	}

	if events[1].Type != "run.completed" {
		t.Fatalf("events[1].Type = %q, want %q", events[1].Type, "run.completed")
	}
	if events[1].RunID != runID {
		t.Fatalf("events[1].RunID = %q, want %q", events[1].RunID, runID)
	}
	if events[1].MessageID != assistantMessageID {
		t.Fatalf("events[1].MessageID = %q, want %q", events[1].MessageID, assistantMessageID)
	}
	if events[1].ResponseID != providerResponseID {
		t.Fatalf("events[1].ResponseID = %q, want %q", events[1].ResponseID, providerResponseID)
	}
	if events[1].ModelName != modelName {
		t.Fatalf("events[1].ModelName = %q, want %q", events[1].ModelName, modelName)
	}
	if events[1].InputTokens == nil || *events[1].InputTokens != inputTokens {
		t.Fatalf("events[1].InputTokens = %v, want %d", events[1].InputTokens, inputTokens)
	}
	if events[1].OutputTokens == nil || *events[1].OutputTokens != outputTokens {
		t.Fatalf("events[1].OutputTokens = %v, want %d", events[1].OutputTokens, outputTokens)
	}
	if events[1].TotalTokens == nil || *events[1].TotalTokens != totalTokens {
		t.Fatalf("events[1].TotalTokens = %v, want %d", events[1].TotalTokens, totalTokens)
	}
}

func TestDeriveSessionTitle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty string", input: "", want: ""},
		{name: "whitespace only", input: "   \t\n  ", want: ""},
		{name: "simple message", input: "Hello world", want: "Hello world"},
		{name: "normalizes whitespace", input: "  Hello   world  ", want: "Hello world"},
		{name: "multiline takes first sentence", input: "First line.\nSecond line.", want: "First line."},
		{name: "sentence boundary", input: "Ask about Go. Then ask about Rust.", want: "Ask about Go."},
		{name: "sentence at end of string", input: "Ask about Go.", want: "Ask about Go."},
		{name: "no sentence boundary", input: "Ask about Go and Rust", want: "Ask about Go and Rust"},
		{name: "empty input from null bytes", input: "", want: ""},
		{
			name:  "truncates long non-empty text",
			input: "Explain the concept of distributed systems and how they relate to modern computing practices in great detail",
			want:  "Explain the concept of distributed systems and how they relate to modern computi",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := deriveSessionTitle(test.input)
			if got != test.want {
				t.Fatalf("deriveSessionTitle(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}

func TestDeriveSessionTitleTruncatesAt80(t *testing.T) {
	t.Parallel()

	longText := ""
	for i := range 100 {
		longText += string(rune('a' + (i % 26)))
	}

	got := deriveSessionTitle(longText)
	if len(got) != sessionTitleMaxLength {
		t.Fatalf("len(deriveSessionTitle) = %d, want %d", len(got), sessionTitleMaxLength)
	}
}
