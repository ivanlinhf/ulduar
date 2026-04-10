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
	"github.com/ivanlin/ulduar/apps/backend/internal/imagegen"
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
		`"toolPhase":"searching"`,
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

func TestShouldBypassTimeoutBuffering(t *testing.T) {
	tests := []struct {
		name string
		req  *http.Request
		want bool
	}{
		{
			name: "image generation asset content",
			req:  httptest.NewRequest(http.MethodGet, "/api/v1/sessions/11111111-1111-1111-1111-111111111111/image-generations/55555555-5555-5555-5555-555555555555/assets/77777777-7777-7777-7777-777777777777/content", nil),
			want: true,
		},
		{
			name: "image generation image content",
			req:  httptest.NewRequest(http.MethodGet, "/api/v1/sessions/11111111-1111-1111-1111-111111111111/image-generations/55555555-5555-5555-5555-555555555555/images/77777777-7777-7777-7777-777777777777/content", nil),
			want: true,
		},
		{
			name: "image generation detail",
			req:  httptest.NewRequest(http.MethodGet, "/api/v1/sessions/11111111-1111-1111-1111-111111111111/image-generations/55555555-5555-5555-5555-555555555555", nil),
			want: false,
		},
		{
			name: "stream route",
			req:  httptest.NewRequest(http.MethodGet, "/api/v1/sessions/11111111-1111-1111-1111-111111111111/image-generations/55555555-5555-5555-5555-555555555555/stream", nil),
			want: false,
		},
		{
			name: "asset content prefix only",
			req:  httptest.NewRequest(http.MethodGet, "/api/v1/sessions/11111111-1111-1111-1111-111111111111/image-generations/55555555-5555-5555-5555-555555555555/assets/77777777-7777-7777-7777-777777777777/content/other", nil),
			want: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := shouldBypassTimeoutBuffering(test.req); got != test.want {
				t.Fatalf("shouldBypassTimeoutBuffering() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestImageGenerationCapabilitiesHandler(t *testing.T) {
	service := &fakeImageGenerationService{
		providerConfigured: true,
		capabilities: imagegen.Capabilities{
			Modes:              []imagegen.Mode{imagegen.ModeTextToImage, imagegen.ModeImageEdit},
			Resolutions:        []imagegen.Resolution{{Key: "1024x1024", Width: 1024, Height: 1024}},
			MaxReferenceImages: imagegen.MaxReferenceImages,
			OutputImageCount:   imagegen.OutputImageCountV1,
			ProviderName:       "azure_foundry",
		},
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/image-generations/capabilities", nil)

	NewHandler(&fakeChatService{}, HandlerOptions{ImageGenerationService: service}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var payload imageGenerationCapabilitiesResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Modes) != 2 || payload.Modes[0] != "text_to_image" || payload.Modes[1] != "image_edit" {
		t.Fatalf("payload.Modes = %#v", payload.Modes)
	}
	if payload.ProviderName != "azure_foundry" {
		t.Fatalf("payload.ProviderName = %q", payload.ProviderName)
	}
}

func TestImageGenerationCapabilitiesHandlerReturnsServiceUnavailableWithoutProvider(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/image-generations/capabilities", nil)

	NewHandler(&fakeChatService{}, HandlerOptions{
		ImageGenerationService: &fakeImageGenerationService{},
	}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}

	var payload errorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Error != "image generation is not configured" {
		t.Fatalf("payload.Error = %q", payload.Error)
	}
}

func TestImageGenerationCapabilitiesHandlerReturnsServiceUnavailableWithoutService(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/image-generations/capabilities", nil)

	NewHandler(&fakeChatService{}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
}

func TestCreateImageGenerationHandlerJSONFlow(t *testing.T) {
	var received imagegen.CreateGenerationParams
	service := &fakeImageGenerationService{
		providerConfigured: true,
		createPendingGenerationFn: func(_ context.Context, params imagegen.CreateGenerationParams) (imagegen.GenerationView, error) {
			received = params
			return imagegen.GenerationView{
				Generation: imagegen.Generation{
					ID:        "55555555-5555-5555-5555-555555555555",
					SessionID: params.SessionID,
					Status:    imagegen.StatusPending,
					CreatedAt: time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC),
				},
			}, nil
		},
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/sessions/11111111-1111-1111-1111-111111111111/image-generations",
		strings.NewReader(`{"mode":"text_to_image","prompt":"draw a fox","resolution":"1024x1024"}`),
	)
	request.Header.Set("Content-Type", "application/json")

	NewHandler(&fakeChatService{}, HandlerOptions{ImageGenerationService: service}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusAccepted)
	}
	if received.SessionID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("received.SessionID = %q", received.SessionID)
	}
	if received.Mode != imagegen.ModeTextToImage {
		t.Fatalf("received.Mode = %q", received.Mode)
	}
	if received.Prompt != "draw a fox" {
		t.Fatalf("received.Prompt = %q", received.Prompt)
	}
	if received.ResolutionKey != "1024x1024" {
		t.Fatalf("received.ResolutionKey = %q", received.ResolutionKey)
	}

	var payload createImageGenerationResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.GenerationID != "55555555-5555-5555-5555-555555555555" {
		t.Fatalf("payload.GenerationID = %q", payload.GenerationID)
	}
}

func TestCreateImageGenerationHandlerDefaultsMissingContentTypeToJSON(t *testing.T) {
	var received imagegen.CreateGenerationParams
	service := &fakeImageGenerationService{
		providerConfigured: true,
		createPendingGenerationFn: func(_ context.Context, params imagegen.CreateGenerationParams) (imagegen.GenerationView, error) {
			received = params
			return imagegen.GenerationView{
				Generation: imagegen.Generation{
					ID:        "55555555-5555-5555-5555-555555555555",
					SessionID: params.SessionID,
					Status:    imagegen.StatusPending,
					CreatedAt: time.Date(2026, 4, 10, 9, 1, 0, 0, time.UTC),
				},
			}, nil
		},
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/sessions/11111111-1111-1111-1111-111111111111/image-generations",
		strings.NewReader(`{"mode":"text_to_image","prompt":"draw a fox","resolution":"1024x1024"}`),
	)

	NewHandler(&fakeChatService{}, HandlerOptions{ImageGenerationService: service}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusAccepted)
	}
	if received.Mode != imagegen.ModeTextToImage {
		t.Fatalf("received.Mode = %q", received.Mode)
	}
}

func TestCreateImageGenerationHandlerReturnsServiceUnavailableWithoutProvider(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/sessions/11111111-1111-1111-1111-111111111111/image-generations",
		strings.NewReader(`{"mode":"text_to_image","prompt":"draw a fox","resolution":"1024x1024"}`),
	)
	request.Header.Set("Content-Type", "application/json")

	NewHandler(&fakeChatService{}, HandlerOptions{
		ImageGenerationService: &fakeImageGenerationService{},
	}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
}

func TestCreateImageGenerationHandlerReturnsServiceUnavailableWithoutService(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/sessions/11111111-1111-1111-1111-111111111111/image-generations",
		strings.NewReader(`{"mode":"text_to_image","prompt":"draw a fox","resolution":"1024x1024"}`),
	)
	request.Header.Set("Content-Type", "application/json")

	NewHandler(&fakeChatService{}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
}

func TestCreateImageGenerationHandlerMultipartFlow(t *testing.T) {
	var received imagegen.CreateGenerationParams
	service := &fakeImageGenerationService{
		providerConfigured: true,
		createPendingGenerationFn: func(_ context.Context, params imagegen.CreateGenerationParams) (imagegen.GenerationView, error) {
			received = params
			return imagegen.GenerationView{
				Generation: imagegen.Generation{
					ID:        "55555555-5555-5555-5555-555555555555",
					SessionID: params.SessionID,
					Status:    imagegen.StatusPending,
					CreatedAt: time.Date(2026, 4, 10, 9, 5, 0, 0, time.UTC),
				},
			}, nil
		},
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("mode", "image_edit"); err != nil {
		t.Fatalf("WriteField(mode): %v", err)
	}
	if err := writer.WriteField("prompt", "make it warmer"); err != nil {
		t.Fatalf("WriteField(prompt): %v", err)
	}
	if err := writer.WriteField("resolution", "1024x1024"); err != nil {
		t.Fatalf("WriteField(resolution): %v", err)
	}
	part, err := writer.CreateFormFile("referenceImages", "input.png")
	if err != nil {
		t.Fatalf("CreateFormFile(): %v", err)
	}
	if _, err := io.Copy(part, bytes.NewReader(testPNGData())); err != nil {
		t.Fatalf("Write reference image: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close(): %v", err)
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/sessions/11111111-1111-1111-1111-111111111111/image-generations",
		body,
	)
	request.Header.Set("Content-Type", writer.FormDataContentType())

	recorder := httptest.NewRecorder()
	NewHandler(&fakeChatService{}, HandlerOptions{ImageGenerationService: service}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusAccepted)
	}
	if received.Mode != imagegen.ModeImageEdit {
		t.Fatalf("received.Mode = %q", received.Mode)
	}
	if len(received.ReferenceImages) != 1 {
		t.Fatalf("len(received.ReferenceImages) = %d", len(received.ReferenceImages))
	}
	if received.ReferenceImages[0].Filename != "input.png" {
		t.Fatalf("reference image filename = %q", received.ReferenceImages[0].Filename)
	}
}

func TestCreateImageGenerationHandlerRejectsImageEditJSON(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/sessions/11111111-1111-1111-1111-111111111111/image-generations",
		strings.NewReader(`{"mode":"image_edit","prompt":"edit this","resolution":"1024x1024"}`),
	)
	request.Header.Set("Content-Type", "application/json")

	NewHandler(&fakeChatService{}, HandlerOptions{
		ImageGenerationService: &fakeImageGenerationService{providerConfigured: true},
	}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}

	var payload errorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Error != "image_edit requests must use multipart/form-data" {
		t.Fatalf("payload.Error = %q", payload.Error)
	}
}

func TestGetImageGenerationHandlerIncludesScopedOutputContentURL(t *testing.T) {
	service := &fakeImageGenerationService{
		providerConfigured: true,
		getGenerationFn: func(_ context.Context, sessionID, generationID string) (imagegen.GenerationView, error) {
			if sessionID != "11111111-1111-1111-1111-111111111111" {
				t.Fatalf("sessionID = %q", sessionID)
			}
			if generationID != "55555555-5555-5555-5555-555555555555" {
				t.Fatalf("generationID = %q", generationID)
			}
			return imagegen.GenerationView{
				Generation: imagegen.Generation{
					ID:               generationID,
					SessionID:        sessionID,
					Mode:             imagegen.ModeImageEdit,
					Status:           imagegen.StatusCompleted,
					Prompt:           "make it warmer",
					Resolution:       imagegen.Resolution{Key: "1024x1024", Width: 1024, Height: 1024},
					OutputImageCount: 1,
					CreatedAt:        time.Date(2026, 4, 10, 9, 10, 0, 0, time.UTC),
				},
				Assets: []imagegen.Asset{
					{
						ID:           "66666666-6666-6666-6666-666666666666",
						GenerationID: generationID,
						Role:         imagegen.AssetRoleInput,
						Filename:     "input.png",
						MediaType:    "image/png",
						SizeBytes:    68,
						SHA256:       "input-sha",
						CreatedAt:    time.Date(2026, 4, 10, 9, 10, 0, 0, time.UTC),
					},
					{
						ID:           "77777777-7777-7777-7777-777777777777",
						GenerationID: generationID,
						Role:         imagegen.AssetRoleOutput,
						Filename:     "output.png",
						MediaType:    "image/png",
						SizeBytes:    68,
						SHA256:       "output-sha",
						CreatedAt:    time.Date(2026, 4, 10, 9, 11, 0, 0, time.UTC),
					},
				},
			}, nil
		},
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/sessions/11111111-1111-1111-1111-111111111111/image-generations/55555555-5555-5555-5555-555555555555",
		nil,
	)

	NewHandler(&fakeChatService{}, HandlerOptions{ImageGenerationService: service}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var payload imageGenerationResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.InputAssets) != 1 {
		t.Fatalf("len(payload.InputAssets) = %d", len(payload.InputAssets))
	}
	if len(payload.OutputAssets) != 1 {
		t.Fatalf("len(payload.OutputAssets) = %d", len(payload.OutputAssets))
	}
	wantURL := "/api/v1/sessions/11111111-1111-1111-1111-111111111111/image-generations/55555555-5555-5555-5555-555555555555/images/77777777-7777-7777-7777-777777777777/content"
	if payload.OutputAssets[0].ContentURL != wantURL {
		t.Fatalf("payload.OutputAssets[0].ContentURL = %q, want %q", payload.OutputAssets[0].ContentURL, wantURL)
	}
}

func TestGetImageGenerationAssetContentHandler(t *testing.T) {
	service := &fakeImageGenerationService{
		providerConfigured: true,
		getAssetContentFn: func(_ context.Context, sessionID, generationID, assetID string) (imagegen.AssetContent, error) {
			if sessionID != "11111111-1111-1111-1111-111111111111" {
				t.Fatalf("sessionID = %q", sessionID)
			}
			if generationID != "55555555-5555-5555-5555-555555555555" {
				t.Fatalf("generationID = %q", generationID)
			}
			if assetID != "77777777-7777-7777-7777-777777777777" {
				t.Fatalf("assetID = %q", assetID)
			}
			return imagegen.AssetContent{
				Filename:  "output.png",
				MediaType: "image/png",
				Data:      testPNGData(),
			}, nil
		},
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/sessions/11111111-1111-1111-1111-111111111111/image-generations/55555555-5555-5555-5555-555555555555/assets/77777777-7777-7777-7777-777777777777/content",
		nil,
	)

	NewHandler(&fakeChatService{}, HandlerOptions{ImageGenerationService: service}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := recorder.Header().Get("Content-Disposition"); !strings.Contains(got, "output.png") {
		t.Fatalf("Content-Disposition = %q", got)
	}
	if !bytes.Equal(recorder.Body.Bytes(), testPNGData()) {
		t.Fatalf("body bytes mismatch")
	}
}

func TestGetImageGenerationImageContentHandlerSuccess(t *testing.T) {
	service := &fakeImageGenerationService{
		providerConfigured: true,
		getImageContentFn: func(_ context.Context, sessionID, generationID, imageID string) (imagegen.AssetContent, error) {
			if sessionID != "11111111-1111-1111-1111-111111111111" {
				t.Fatalf("sessionID = %q", sessionID)
			}
			if generationID != "55555555-5555-5555-5555-555555555555" {
				t.Fatalf("generationID = %q", generationID)
			}
			if imageID != "77777777-7777-7777-7777-777777777777" {
				t.Fatalf("imageID = %q", imageID)
			}
			return imagegen.AssetContent{
				Filename:  "output.png",
				MediaType: "image/png",
				Data:      testPNGData(),
			}, nil
		},
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/sessions/11111111-1111-1111-1111-111111111111/image-generations/55555555-5555-5555-5555-555555555555/images/77777777-7777-7777-7777-777777777777/content",
		nil,
	)

	NewHandler(&fakeChatService{}, HandlerOptions{ImageGenerationService: service}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if got := recorder.Header().Get("Content-Disposition"); !strings.Contains(got, "output.png") {
		t.Fatalf("Content-Disposition = %q", got)
	}
	if !bytes.Equal(recorder.Body.Bytes(), testPNGData()) {
		t.Fatalf("body bytes mismatch")
	}
}

func TestGetImageGenerationImageContentHandlerNotFound(t *testing.T) {
	service := &fakeImageGenerationService{
		providerConfigured: true,
		getImageContentFn: func(_ context.Context, _, _, _ string) (imagegen.AssetContent, error) {
			return imagegen.AssetContent{}, imagegen.ValidationError{StatusCode: http.StatusNotFound, Message: "image not found"}
		},
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/sessions/11111111-1111-1111-1111-111111111111/image-generations/55555555-5555-5555-5555-555555555555/images/99999999-9999-9999-9999-999999999999/content",
		nil,
	)

	NewHandler(&fakeChatService{}, HandlerOptions{ImageGenerationService: service}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q", got)
	}
}

func TestGetImageGenerationImageContentHandlerMismatchedIDs(t *testing.T) {
	service := &fakeImageGenerationService{
		providerConfigured: true,
		getImageContentFn: func(_ context.Context, _, _, _ string) (imagegen.AssetContent, error) {
			return imagegen.AssetContent{}, imagegen.ValidationError{StatusCode: http.StatusNotFound, Message: "image not found"}
		},
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/sessions/11111111-1111-1111-1111-111111111111/image-generations/55555555-5555-5555-5555-555555555555/images/77777777-7777-7777-7777-777777777777/content",
		nil,
	)

	NewHandler(&fakeChatService{}, HandlerOptions{ImageGenerationService: service}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

func TestGetImageGenerationImageContentHandlerUnavailableWithoutService(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/sessions/11111111-1111-1111-1111-111111111111/image-generations/55555555-5555-5555-5555-555555555555/images/77777777-7777-7777-7777-777777777777/content",
		nil,
	)

	NewHandler(&fakeChatService{}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
}

func TestGetImageGenerationImageContentHandlerInvalidImageID(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/sessions/11111111-1111-1111-1111-111111111111/image-generations/55555555-5555-5555-5555-555555555555/images/not-a-uuid/content",
		nil,
	)

	service := &fakeImageGenerationService{providerConfigured: true}
	NewHandler(&fakeChatService{}, HandlerOptions{ImageGenerationService: service}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "imageId") {
		t.Fatalf("error response missing imageId field name: %s", body)
	}
}

func TestStreamImageGenerationHandlerSuccessFlow(t *testing.T) {
	getCalls := 0
	service := &fakeImageGenerationService{
		providerConfigured: true,
		getGenerationFn: func(_ context.Context, _, generationID string) (imagegen.GenerationView, error) {
			getCalls++
			switch getCalls {
			case 1:
				return testGenerationView(generationID, imagegen.StatusPending), nil
			case 2:
				return testGenerationView(generationID, imagegen.StatusRunning), nil
			default:
				view := testGenerationView(generationID, imagegen.StatusCompleted)
				view.Assets = []imagegen.Asset{{
					ID:           "77777777-7777-7777-7777-777777777777",
					GenerationID: generationID,
					Role:         imagegen.AssetRoleOutput,
					Filename:     "output.png",
					MediaType:    "image/png",
					SizeBytes:    68,
					SHA256:       "output-sha",
					CreatedAt:    time.Date(2026, 4, 10, 9, 12, 0, 0, time.UTC),
				}}
				completedAt := time.Date(2026, 4, 10, 9, 12, 0, 0, time.UTC)
				view.Generation.CompletedAt = &completedAt
				return view, nil
			}
		},
		executeGenerationFn: func(context.Context, string) error { return nil },
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/sessions/11111111-1111-1111-1111-111111111111/image-generations/55555555-5555-5555-5555-555555555555/stream",
		nil,
	)

	NewHandler(&fakeChatService{}, HandlerOptions{
		ImageGenerationService:      service,
		ImageGenerationPollInterval: time.Millisecond,
	}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	body := recorder.Body.String()
	for _, fragment := range []string{
		"event: image_generation.started",
		"event: image_generation.running",
		"event: image_generation.completed",
		`"generationId":"55555555-5555-5555-5555-555555555555"`,
		`"contentUrl":"/api/v1/sessions/11111111-1111-1111-1111-111111111111/image-generations/55555555-5555-5555-5555-555555555555/images/77777777-7777-7777-7777-777777777777/content"`,
	} {
		if !strings.Contains(body, fragment) {
			t.Fatalf("stream body missing %q:\n%s", fragment, body)
		}
	}
}

func TestStreamImageGenerationHandlerFailureFlow(t *testing.T) {
	getCalls := 0
	service := &fakeImageGenerationService{
		providerConfigured: true,
		getGenerationFn: func(_ context.Context, _, generationID string) (imagegen.GenerationView, error) {
			getCalls++
			view := testGenerationView(generationID, imagegen.StatusPending)
			if getCalls > 1 {
				view.Generation.Status = imagegen.StatusFailed
				view.Generation.ErrorCode = "provider_request_failed"
				view.Generation.ErrorMessage = "provider request failed"
			}
			return view, nil
		},
		executeGenerationFn: func(context.Context, string) error { return errors.New("provider request failed") },
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/sessions/11111111-1111-1111-1111-111111111111/image-generations/55555555-5555-5555-5555-555555555555/stream",
		nil,
	)

	NewHandler(&fakeChatService{}, HandlerOptions{ImageGenerationService: service}).ServeHTTP(recorder, request)

	body := recorder.Body.String()
	for _, fragment := range []string{
		"event: image_generation.started",
		"event: image_generation.failed",
		`"errorCode":"provider_request_failed"`,
		`"errorMessage":"provider request failed"`,
	} {
		if !strings.Contains(body, fragment) {
			t.Fatalf("stream body missing %q:\n%s", fragment, body)
		}
	}
}

func TestStreamImageGenerationHandlerSuppressesSyntheticFailureAfterStreamStarts(t *testing.T) {
	service := &fakeImageGenerationService{
		providerConfigured: true,
		getGenerationFn: func(_ context.Context, _, generationID string) (imagegen.GenerationView, error) {
			return testGenerationView(generationID, imagegen.StatusPending), nil
		},
		executeGenerationFn: func(context.Context, string) error { return errors.New("provider unavailable") },
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/sessions/11111111-1111-1111-1111-111111111111/image-generations/55555555-5555-5555-5555-555555555555/stream",
		nil,
	)

	NewHandler(&fakeChatService{}, HandlerOptions{ImageGenerationService: service}).ServeHTTP(recorder, request)

	body := recorder.Body.String()
	if !strings.Contains(body, "event: image_generation.started") {
		t.Fatalf("stream body missing started event:\n%s", body)
	}
	if strings.Contains(body, "event: image_generation.failed") {
		t.Fatalf("unexpected synthetic failed event:\n%s", body)
	}
	if strings.Contains(body, "internal server error") {
		t.Fatalf("unexpected JSON error in stream body:\n%s", body)
	}
}

func TestStreamImageGenerationHandlerAlreadyCompleted(t *testing.T) {
	executeCalls := 0
	service := &fakeImageGenerationService{
		providerConfigured: true,
		getGenerationFn: func(_ context.Context, _, generationID string) (imagegen.GenerationView, error) {
			return testGenerationView(generationID, imagegen.StatusCompleted), nil
		},
		executeGenerationFn: func(context.Context, string) error {
			executeCalls++
			return nil
		},
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/sessions/11111111-1111-1111-1111-111111111111/image-generations/55555555-5555-5555-5555-555555555555/stream",
		nil,
	)

	NewHandler(&fakeChatService{}, HandlerOptions{ImageGenerationService: service}).ServeHTTP(recorder, request)

	if executeCalls != 0 {
		t.Fatalf("executeCalls = %d, want 0", executeCalls)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "event: image_generation.completed") {
		t.Fatalf("stream body missing completed event:\n%s", body)
	}
	if strings.Contains(body, "event: image_generation.started") {
		t.Fatalf("unexpected started event:\n%s", body)
	}
}

func TestStreamImageGenerationHandlerReturnsServiceUnavailableWhenProviderMissing(t *testing.T) {
	service := &fakeImageGenerationService{
		getGenerationFn: func(_ context.Context, _, generationID string) (imagegen.GenerationView, error) {
			return testGenerationView(generationID, imagegen.StatusPending), nil
		},
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/sessions/11111111-1111-1111-1111-111111111111/image-generations/55555555-5555-5555-5555-555555555555/stream",
		nil,
	)

	NewHandler(&fakeChatService{}, HandlerOptions{ImageGenerationService: service}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q", got)
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

type fakeImageGenerationService struct {
	providerConfigured        bool
	capabilities              imagegen.Capabilities
	createPendingGenerationFn func(ctx context.Context, params imagegen.CreateGenerationParams) (imagegen.GenerationView, error)
	getGenerationFn           func(ctx context.Context, sessionID, generationID string) (imagegen.GenerationView, error)
	executeGenerationFn       func(ctx context.Context, generationID string) error
	getAssetContentFn         func(ctx context.Context, sessionID, generationID, assetID string) (imagegen.AssetContent, error)
	getImageContentFn         func(ctx context.Context, sessionID, generationID, imageID string) (imagegen.AssetContent, error)
}

func (f *fakeImageGenerationService) Capabilities() imagegen.Capabilities {
	return f.capabilities
}

func (f *fakeImageGenerationService) ProviderConfigured() bool {
	return f.providerConfigured
}

func (f *fakeImageGenerationService) CreatePendingGeneration(ctx context.Context, params imagegen.CreateGenerationParams) (imagegen.GenerationView, error) {
	if f.createPendingGenerationFn == nil {
		return imagegen.GenerationView{}, errors.New("unexpected CreatePendingGeneration call")
	}
	return f.createPendingGenerationFn(ctx, params)
}

func (f *fakeImageGenerationService) GetGeneration(ctx context.Context, sessionID, generationID string) (imagegen.GenerationView, error) {
	if f.getGenerationFn == nil {
		return imagegen.GenerationView{}, errors.New("unexpected GetGeneration call")
	}
	return f.getGenerationFn(ctx, sessionID, generationID)
}

func (f *fakeImageGenerationService) ExecuteGeneration(ctx context.Context, generationID string) error {
	if f.executeGenerationFn == nil {
		return errors.New("unexpected ExecuteGeneration call")
	}
	return f.executeGenerationFn(ctx, generationID)
}

func (f *fakeImageGenerationService) GetAssetContent(ctx context.Context, sessionID, generationID, assetID string) (imagegen.AssetContent, error) {
	if f.getAssetContentFn == nil {
		return imagegen.AssetContent{}, errors.New("unexpected GetAssetContent call")
	}
	return f.getAssetContentFn(ctx, sessionID, generationID, assetID)
}

func (f *fakeImageGenerationService) GetImageContent(ctx context.Context, sessionID, generationID, imageID string) (imagegen.AssetContent, error) {
	if f.getImageContentFn == nil {
		return imagegen.AssetContent{}, errors.New("unexpected GetImageContent call")
	}
	return f.getImageContentFn(ctx, sessionID, generationID, imageID)
}

func testGenerationView(generationID string, status imagegen.Status) imagegen.GenerationView {
	return imagegen.GenerationView{
		Generation: imagegen.Generation{
			ID:               generationID,
			SessionID:        "11111111-1111-1111-1111-111111111111",
			Mode:             imagegen.ModeTextToImage,
			Status:           status,
			Prompt:           "draw a fox",
			Resolution:       imagegen.Resolution{Key: "1024x1024", Width: 1024, Height: 1024},
			OutputImageCount: 1,
			CreatedAt:        time.Date(2026, 4, 10, 9, 10, 0, 0, time.UTC),
		},
	}
}

func testPNGData() []byte {
	return []byte{
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
}
