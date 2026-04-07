package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ivanlin/ulduar/apps/backend/internal/chat"
	"github.com/ivanlin/ulduar/apps/backend/internal/repository"
)

func TestCreateSessionHandler(t *testing.T) {
	service := &fakeChatService{
		createSessionFn: func(context.Context) (repository.Session, error) {
			now := time.Date(2026, 3, 31, 9, 0, 0, 0, time.UTC)
			return repository.Session{
				ID:            "11111111-1111-1111-1111-111111111111",
				Status:        "active",
				CreatedAt:     now,
				LastMessageAt: now,
			}, nil
		},
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", nil)

	NewHandler(service).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusCreated)
	}

	var payload sessionResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if payload.SessionID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("payload.SessionID = %q", payload.SessionID)
	}
	if got := recorder.Header().Get("X-Request-Id"); got == "" {
		t.Fatal("X-Request-Id header is empty")
	}
}

func TestCreateMessageHandlerJSONValidation(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        string
		statusCode  int
		wantError   string
	}{
		{
			name:        "unknown field",
			contentType: "application/json",
			body:        `{"text":"hello","extra":true}`,
			statusCode:  http.StatusBadRequest,
			wantError:   `json: unknown field "extra"`,
		},
		{
			name:        "invalid json",
			contentType: "application/json",
			body:        `{"text":`,
			statusCode:  http.StatusBadRequest,
			wantError:   "request body contains invalid JSON",
		},
		{
			name:        "empty body",
			contentType: "application/json",
			body:        "",
			statusCode:  http.StatusBadRequest,
			wantError:   "request body is required",
		},
		{
			name:        "unsupported media type",
			contentType: "text/plain",
			body:        "hello",
			statusCode:  http.StatusUnsupportedMediaType,
			wantError:   "Content-Type must be application/json or multipart/form-data",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(
				http.MethodPost,
				"/api/v1/sessions/11111111-1111-1111-1111-111111111111/messages",
				strings.NewReader(test.body),
			)
			request.Header.Set("Content-Type", test.contentType)

			NewHandler(&fakeChatService{}).ServeHTTP(recorder, request)

			if recorder.Code != test.statusCode {
				t.Fatalf("status = %d, want %d", recorder.Code, test.statusCode)
			}

			var payload errorResponse
			if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode error response: %v", err)
			}
			if payload.Error != test.wantError {
				t.Fatalf("payload.Error = %q, want %q", payload.Error, test.wantError)
			}
			if payload.Code == "" {
				t.Fatal("payload.Code is empty")
			}
			if payload.RequestID == "" {
				t.Fatal("payload.RequestID is empty")
			}
		})
	}
}

func TestRequestIDMiddlewareUsesIncomingHeader(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/sessions/11111111-1111-1111-1111-111111111111/messages",
		strings.NewReader(`{"text":"hello","extra":true}`),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-Id", "req-test-123")

	NewHandler(&fakeChatService{}).ServeHTTP(recorder, request)

	if got := recorder.Header().Get("X-Request-Id"); got != "req-test-123" {
		t.Fatalf("X-Request-Id = %q", got)
	}

	var payload errorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if payload.RequestID != "req-test-123" {
		t.Fatalf("payload.RequestID = %q", payload.RequestID)
	}
}

func TestCreateMessageHandlerJSONFlow(t *testing.T) {
	var received chat.CreateMessageParams
	service := &fakeChatService{
		createMessageFn: func(_ context.Context, params chat.CreateMessageParams) (chat.MessageCreation, error) {
			received = params
			startedAt := time.Date(2026, 3, 31, 9, 30, 0, 0, time.UTC)
			return chat.MessageCreation{
				UserMessage: repository.Message{ID: "22222222-2222-2222-2222-222222222222"},
				AssistantMessage: repository.Message{
					ID: "33333333-3333-3333-3333-333333333333",
				},
				Run: repository.Run{
					ID:        "44444444-4444-4444-4444-444444444444",
					StartedAt: startedAt,
				},
			}, nil
		},
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/sessions/11111111-1111-1111-1111-111111111111/messages",
		strings.NewReader(`{"text":"hello world"}`),
	)
	request.Header.Set("Content-Type", "application/json")

	NewHandler(service).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusAccepted)
	}
	if received.SessionID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("received.SessionID = %q", received.SessionID)
	}
	if received.Text != "hello world" {
		t.Fatalf("received.Text = %q", received.Text)
	}

	var payload createMessageResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.RunID != "44444444-4444-4444-4444-444444444444" {
		t.Fatalf("payload.RunID = %q", payload.RunID)
	}
}

func TestCreateMessageHandlerMultipartFlow(t *testing.T) {
	var received chat.CreateMessageParams
	service := &fakeChatService{
		createMessageFn: func(_ context.Context, params chat.CreateMessageParams) (chat.MessageCreation, error) {
			received = params
			return chat.MessageCreation{
				UserMessage:      repository.Message{ID: "22222222-2222-2222-2222-222222222222"},
				AssistantMessage: repository.Message{ID: "33333333-3333-3333-3333-333333333333"},
				Run: repository.Run{
					ID:        "44444444-4444-4444-4444-444444444444",
					StartedAt: time.Date(2026, 3, 31, 9, 45, 0, 0, time.UTC),
				},
			}, nil
		},
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("text", "summarize this"); err != nil {
		t.Fatalf("WriteField(): %v", err)
	}
	part, err := writer.CreateFormFile("attachments", "diagram.png")
	if err != nil {
		t.Fatalf("CreateFormFile(): %v", err)
	}
	png := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
		0x89, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x44, 0x41,
		0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00,
		0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00,
		0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae,
		0x42, 0x60, 0x82,
	}
	if _, err := io.Copy(part, bytes.NewReader(png)); err != nil {
		t.Fatalf("Write attachment: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close(): %v", err)
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/sessions/11111111-1111-1111-1111-111111111111/messages",
		body,
	)
	request.Header.Set("Content-Type", writer.FormDataContentType())

	recorder := httptest.NewRecorder()
	NewHandler(service).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusAccepted)
	}
	if received.Text != "summarize this" {
		t.Fatalf("received.Text = %q", received.Text)
	}
	if len(received.Attachments) != 1 {
		t.Fatalf("len(received.Attachments) = %d", len(received.Attachments))
	}
	if received.Attachments[0].Filename != "diagram.png" {
		t.Fatalf("attachment filename = %q", received.Attachments[0].Filename)
	}
}

func TestGetSessionHandlerIncludesAssistantTokenUsage(t *testing.T) {
	inputTokens := int64(45)
	outputTokens := int64(78)
	totalTokens := int64(123)
	createdAt := time.Date(2026, 3, 31, 9, 15, 0, 0, time.UTC)

	service := &fakeChatService{
		getSessionFn: func(context.Context, string) (chat.SessionView, error) {
			return chat.SessionView{
				Session: repository.Session{
					ID:            "11111111-1111-1111-1111-111111111111",
					Status:        "active",
					CreatedAt:     createdAt,
					LastMessageAt: createdAt,
				},
				Messages: []chat.MessageView{
					{
						Message: repository.Message{
							ID:        "33333333-3333-3333-3333-333333333333",
							Role:      "assistant",
							Status:    "completed",
							ModelName: "gpt-5",
							CreatedAt: createdAt,
						},
						Content: chat.MessageContent{
							Parts: []chat.MessageContentPart{{Type: "text", Text: "Assistant reply"}},
						},
						TokenUsage: &chat.TokenUsage{
							InputTokens:  &inputTokens,
							OutputTokens: &outputTokens,
							TotalTokens:  &totalTokens,
						},
					},
				},
			}, nil
		},
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/11111111-1111-1111-1111-111111111111", nil)

	NewHandler(service).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var payload sessionDetailResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Messages) != 1 {
		t.Fatalf("len(payload.Messages) = %d", len(payload.Messages))
	}
	if payload.Messages[0].InputTokens == nil || *payload.Messages[0].InputTokens != 45 {
		t.Fatalf("payload.Messages[0].InputTokens = %v", payload.Messages[0].InputTokens)
	}
	if payload.Messages[0].OutputTokens == nil || *payload.Messages[0].OutputTokens != 78 {
		t.Fatalf("payload.Messages[0].OutputTokens = %v", payload.Messages[0].OutputTokens)
	}
	if payload.Messages[0].TotalTokens == nil || *payload.Messages[0].TotalTokens != 123 {
		t.Fatalf("payload.Messages[0].TotalTokens = %v", payload.Messages[0].TotalTokens)
	}
}

func TestStreamRunHandlerWritesSSEEvents(t *testing.T) {
	inputTokens := int64(45)
	outputTokens := int64(78)
	totalTokens := int64(123)
	service := &fakeChatService{
		streamRunFn: func(_ context.Context, sessionID, runID string, emit func(chat.RunStreamEvent) error) error {
			if sessionID != "11111111-1111-1111-1111-111111111111" {
				t.Fatalf("sessionID = %q", sessionID)
			}
			if runID != "44444444-4444-4444-4444-444444444444" {
				t.Fatalf("runID = %q", runID)
			}

			if err := emit(chat.RunStreamEvent{
				Type:      "tool.status",
				RunID:     runID,
				MessageID: "33333333-3333-3333-3333-333333333333",
				ToolName:  "web_search",
				ToolPhase: "searching",
			}); err != nil {
				return err
			}
			if err := emit(chat.RunStreamEvent{
				Type:       "run.started",
				RunID:      runID,
				MessageID:  "33333333-3333-3333-3333-333333333333",
				ResponseID: "resp_123",
				ModelName:  "gpt-5",
			}); err != nil {
				return err
			}
			if err := emit(chat.RunStreamEvent{
				Type:      "message.delta",
				RunID:     runID,
				MessageID: "33333333-3333-3333-3333-333333333333",
				Delta:     "hello",
			}); err != nil {
				return err
			}
			return emit(chat.RunStreamEvent{
				Type:         "run.completed",
				RunID:        runID,
				MessageID:    "33333333-3333-3333-3333-333333333333",
				ResponseID:   "resp_123",
				ModelName:    "gpt-5",
				InputTokens:  &inputTokens,
				OutputTokens: &outputTokens,
				TotalTokens:  &totalTokens,
			})
		},
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/sessions/11111111-1111-1111-1111-111111111111/runs/44444444-4444-4444-4444-444444444444/stream",
		nil,
	)

	NewHandler(service).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("Content-Type = %q", got)
	}

	body := recorder.Body.String()
	for _, fragment := range []string{
		"event: run.started",
		`"responseId":"resp_123"`,
		"event: tool.status",
		`"toolName":"web_search"`,
		`"phase":"searching"`,
		"event: message.delta",
		`"delta":"hello"`,
		"event: run.completed",
		`"inputTokens":45`,
		`"outputTokens":78`,
		`"totalTokens":123`,
	} {
		if !strings.Contains(body, fragment) {
			t.Fatalf("stream body missing %q:\n%s", fragment, body)
		}
	}
}

func TestStreamRunHandlerReturnsJSONErrorBeforeStreamStarts(t *testing.T) {
	service := &fakeChatService{
		streamRunFn: func(context.Context, string, string, func(chat.RunStreamEvent) error) error {
			return chat.ValidationError{
				StatusCode: http.StatusConflict,
				Message:    "run is already streaming",
			}
		},
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/sessions/11111111-1111-1111-1111-111111111111/runs/44444444-4444-4444-4444-444444444444/stream",
		nil,
	)

	NewHandler(service).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusConflict)
	}

	var payload errorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if payload.Error != "run is already streaming" {
		t.Fatalf("payload.Error = %q", payload.Error)
	}
}

func TestStreamRunHandlerSuppressesJSONErrorAfterStreamStarts(t *testing.T) {
	service := &fakeChatService{
		streamRunFn: func(_ context.Context, _, runID string, emit func(chat.RunStreamEvent) error) error {
			if err := emit(chat.RunStreamEvent{
				Type:      "message.delta",
				RunID:     runID,
				MessageID: "33333333-3333-3333-3333-333333333333",
				Delta:     "partial",
			}); err != nil {
				return err
			}
			return errors.New("provider stream broke")
		},
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/sessions/11111111-1111-1111-1111-111111111111/runs/44444444-4444-4444-4444-444444444444/stream",
		nil,
	)

	NewHandler(service).ServeHTTP(recorder, request)

	body := recorder.Body.String()
	if !strings.Contains(body, "event: message.delta") {
		t.Fatalf("stream body missing message delta:\n%s", body)
	}
	if strings.Contains(body, "internal server error") {
		t.Fatalf("unexpected JSON error in stream body:\n%s", body)
	}
}

func TestRequestTimeoutReturnsStructuredError(t *testing.T) {
	service := &fakeChatService{
		createSessionFn: func(ctx context.Context) (repository.Session, error) {
			<-ctx.Done()
			return repository.Session{}, ctx.Err()
		},
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", nil)

	NewHandler(service, HandlerOptions{
		RequestTimeout:        10 * time.Millisecond,
		MessageRequestTimeout: 10 * time.Millisecond,
	}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusGatewayTimeout)
	}

	var payload errorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if payload.Code != "request_timeout" {
		t.Fatalf("payload.Code = %q", payload.Code)
	}
	if payload.RequestID == "" {
		t.Fatal("payload.RequestID is empty")
	}
}

type fakeChatService struct {
	createSessionFn func(ctx context.Context) (repository.Session, error)
	getSessionFn    func(ctx context.Context, sessionID string) (chat.SessionView, error)
	createMessageFn func(ctx context.Context, params chat.CreateMessageParams) (chat.MessageCreation, error)
	streamRunFn     func(ctx context.Context, sessionID, runID string, emit func(chat.RunStreamEvent) error) error
}

func (f *fakeChatService) CreateSession(ctx context.Context) (repository.Session, error) {
	if f.createSessionFn == nil {
		return repository.Session{}, errors.New("unexpected CreateSession call")
	}
	return f.createSessionFn(ctx)
}

func (f *fakeChatService) GetSession(ctx context.Context, sessionID string) (chat.SessionView, error) {
	if f.getSessionFn == nil {
		return chat.SessionView{}, errors.New("unexpected GetSession call")
	}
	return f.getSessionFn(ctx, sessionID)
}

func (f *fakeChatService) CreateMessage(ctx context.Context, params chat.CreateMessageParams) (chat.MessageCreation, error) {
	if f.createMessageFn == nil {
		return chat.MessageCreation{}, errors.New("unexpected CreateMessage call")
	}
	return f.createMessageFn(ctx, params)
}

func (f *fakeChatService) StreamRun(
	ctx context.Context,
	sessionID, runID string,
	emit func(chat.RunStreamEvent) error,
) error {
	if f.streamRunFn == nil {
		return errors.New("unexpected StreamRun call")
	}
	return f.streamRunFn(ctx, sessionID, runID, emit)
}
