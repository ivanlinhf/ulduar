package httpapi

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/ivanlin/ulduar/apps/backend/internal/chat"
	"github.com/ivanlin/ulduar/apps/backend/internal/repository"
)

type Handler struct {
	chatService chatService
	options     HandlerOptions
}

type HandlerOptions struct {
	RequestTimeout        time.Duration
	MessageRequestTimeout time.Duration
}

type chatService interface {
	CreateSession(ctx context.Context) (repository.Session, error)
	GetSession(ctx context.Context, sessionID string) (chat.SessionView, error)
	CreateMessage(ctx context.Context, params chat.CreateMessageParams) (chat.MessageCreation, error)
	StreamRun(ctx context.Context, sessionID, runID string, emit func(chat.RunStreamEvent) error) error
}

type errorResponse struct {
	Error     string `json:"error"`
	Code      string `json:"code,omitempty"`
	RequestID string `json:"requestId,omitempty"`
}

type healthResponse struct {
	Status string `json:"status"`
}

type sessionResponse struct {
	SessionID     string    `json:"sessionId"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"createdAt"`
	LastMessageAt time.Time `json:"lastMessageAt,omitempty"`
}

type sessionDetailResponse struct {
	SessionID     string            `json:"sessionId"`
	Status        string            `json:"status"`
	CreatedAt     time.Time         `json:"createdAt"`
	LastMessageAt time.Time         `json:"lastMessageAt"`
	Messages      []messageResponse `json:"messages"`
}

type messageResponse struct {
	MessageID    string               `json:"messageId"`
	Role         string               `json:"role"`
	Status       string               `json:"status"`
	ModelName    string               `json:"modelName,omitempty"`
	InputTokens  *int64               `json:"inputTokens,omitempty"`
	OutputTokens *int64               `json:"outputTokens,omitempty"`
	TotalTokens  *int64               `json:"totalTokens,omitempty"`
	CreatedAt    time.Time            `json:"createdAt"`
	Content      chat.MessageContent  `json:"content"`
	Attachments  []attachmentResponse `json:"attachments"`
}

type attachmentResponse struct {
	AttachmentID   string    `json:"attachmentId"`
	Filename       string    `json:"filename"`
	MediaType      string    `json:"mediaType"`
	SizeBytes      int64     `json:"sizeBytes"`
	SHA256         string    `json:"sha256"`
	ProviderFileID string    `json:"providerFileId,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
}

type createMessageResponse struct {
	RunID              string    `json:"runId"`
	UserMessageID      string    `json:"userMessageId"`
	AssistantMessageID string    `json:"assistantMessageId"`
	CreatedAt          time.Time `json:"createdAt"`
}

type createMessageJSONRequest struct {
	Text    string `json:"text"`
	Message string `json:"message"`
}

func NewHandler(chatService chatService, options ...HandlerOptions) http.Handler {
	handlerOptions := HandlerOptions{
		RequestTimeout:        15 * time.Second,
		MessageRequestTimeout: 90 * time.Second,
	}
	if len(options) > 0 {
		handlerOptions = options[0]
	}

	handler := &Handler{
		chatService: chatService,
		options:     handlerOptions,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthHandler)
	mux.HandleFunc("POST /api/v1/sessions", handler.createSessionHandler)
	mux.HandleFunc("GET /api/v1/sessions/{sessionId}", handler.getSessionHandler)
	mux.HandleFunc("POST /api/v1/sessions/{sessionId}/messages", handler.createMessageHandler)
	mux.HandleFunc("GET /api/v1/sessions/{sessionId}/runs/{runId}/stream", handler.streamRunHandler)
	mux.HandleFunc("/", notFoundHandler)

	return withMiddleware(mux, requestIDMiddleware, recoverMiddleware, corsMiddleware, loggingMiddleware, handler.timeoutMiddleware)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{Status: "ok"})
}

func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	writeJSONError(r.Context(), w, http.StatusNotFound, "not_found", "resource not found")
}

func withMiddleware(handler http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	wrapped := handler
	for i := len(middlewares) - 1; i >= 0; i-- {
		wrapped = middlewares[i](wrapped)
	}

	return wrapped
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(recorder, r)
		slog.InfoContext(
			r.Context(),
			"http request completed",
			logFields(
				r.Context(),
				"method", r.Method,
				"path", r.URL.Path,
				"status_code", recorder.statusCode,
				"duration_ms", time.Since(start).Milliseconds(),
				"response_bytes", recorder.bytesWritten,
				"remote_addr", r.RemoteAddr,
			)...,
		)
	})
}

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.TrimSpace(r.Header.Get("X-Request-Id"))
		if requestID == "" {
			requestID = newRequestID()
		}

		ctx := context.WithValue(r.Context(), requestIDContextKey{}, requestID)
		w.Header().Set("X-Request-Id", requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Request-Id")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Expose-Headers", "X-Request-Id")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.ErrorContext(
					r.Context(),
					"panic while serving request",
					logFields(
						r.Context(),
						"method", r.Method,
						"path", r.URL.Path,
						"panic", fmt.Sprint(rec),
						"stack", string(debug.Stack()),
					)...,
				)
				writeJSONError(r.Context(), w, http.StatusInternalServerError, "internal_error", "internal server error")
			}
		}()

		next.ServeHTTP(w, r)
	})
}

func (h *Handler) createSessionHandler(w http.ResponseWriter, r *http.Request) {
	session, err := h.chatService.CreateSession(r.Context())
	if err != nil {
		writeServiceError(r.Context(), w, err)
		return
	}

	writeJSON(w, http.StatusCreated, sessionResponse{
		SessionID:     session.ID,
		Status:        session.Status,
		CreatedAt:     session.CreatedAt,
		LastMessageAt: session.LastMessageAt,
	})
}

func (h *Handler) getSessionHandler(w http.ResponseWriter, r *http.Request) {
	view, err := h.chatService.GetSession(r.Context(), r.PathValue("sessionId"))
	if err != nil {
		writeServiceError(r.Context(), w, err)
		return
	}

	messages := make([]messageResponse, 0, len(view.Messages))
	for _, item := range view.Messages {
		attachments := make([]attachmentResponse, 0, len(item.Attachments))
		for _, attachment := range item.Attachments {
			attachments = append(attachments, attachmentResponse{
				AttachmentID:   attachment.ID,
				Filename:       attachment.Filename,
				MediaType:      attachment.MediaType,
				SizeBytes:      attachment.SizeBytes,
				SHA256:         attachment.Sha256,
				ProviderFileID: attachment.ProviderFileID,
				CreatedAt:      attachment.CreatedAt,
			})
		}

		messages = append(messages, messageResponse{
			MessageID:    item.Message.ID,
			Role:         item.Message.Role,
			Status:       item.Message.Status,
			ModelName:    item.Message.ModelName,
			InputTokens:  tokenUsageField(item.TokenUsage, func(usage *chat.TokenUsage) *int64 { return usage.InputTokens }),
			OutputTokens: tokenUsageField(item.TokenUsage, func(usage *chat.TokenUsage) *int64 { return usage.OutputTokens }),
			TotalTokens:  tokenUsageField(item.TokenUsage, func(usage *chat.TokenUsage) *int64 { return usage.TotalTokens }),
			CreatedAt:    item.Message.CreatedAt,
			Content:      item.Content,
			Attachments:  attachments,
		})
	}

	writeJSON(w, http.StatusOK, sessionDetailResponse{
		SessionID:     view.Session.ID,
		Status:        view.Session.Status,
		CreatedAt:     view.Session.CreatedAt,
		LastMessageAt: view.Session.LastMessageAt,
		Messages:      messages,
	})
}

func (h *Handler) createMessageHandler(w http.ResponseWriter, r *http.Request) {
	params, err := decodeCreateMessageRequest(w, r)
	if err != nil {
		writeServiceError(r.Context(), w, err)
		return
	}

	params.SessionID = r.PathValue("sessionId")

	created, err := h.chatService.CreateMessage(r.Context(), params)
	if err != nil {
		writeServiceError(r.Context(), w, err)
		return
	}

	writeJSON(w, http.StatusAccepted, createMessageResponse{
		RunID:              created.Run.ID,
		UserMessageID:      created.UserMessage.ID,
		AssistantMessageID: created.AssistantMessage.ID,
		CreatedAt:          created.Run.StartedAt,
	})
}

func (h *Handler) streamRunHandler(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSONError(r.Context(), w, http.StatusInternalServerError, "streaming_unsupported", "streaming is not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	streamStarted := false
	err := h.chatService.StreamRun(
		r.Context(),
		r.PathValue("sessionId"),
		r.PathValue("runId"),
		func(event chat.RunStreamEvent) error {
			payload := map[string]any{
				"runId":     event.RunID,
				"messageId": event.MessageID,
			}
			if event.ResponseID != "" {
				payload["responseId"] = event.ResponseID
			}
			if event.ModelName != "" {
				payload["modelName"] = event.ModelName
			}
			if event.Delta != "" {
				payload["delta"] = event.Delta
			}
			if event.Error != "" {
				payload["error"] = event.Error
			}
			if event.ErrorCode != "" {
				payload["errorCode"] = event.ErrorCode
			}
			if event.InputTokens != nil {
				payload["inputTokens"] = *event.InputTokens
			}
			if event.OutputTokens != nil {
				payload["outputTokens"] = *event.OutputTokens
			}
			if event.TotalTokens != nil {
				payload["totalTokens"] = *event.TotalTokens
			}

			if err := writeSSEEvent(w, event.Type, payload); err != nil {
				return err
			}
			streamStarted = true
			flusher.Flush()
			return nil
		},
	)
	if err != nil {
		if streamStarted {
			slog.ErrorContext(
				r.Context(),
				"run stream failed after stream start",
				logFields(r.Context(), "run_id", r.PathValue("runId"), "error", err)...,
			)
			return
		}
		writeServiceError(r.Context(), w, err)
	}
}

func decodeCreateMessageRequest(w http.ResponseWriter, r *http.Request) (chat.CreateMessageParams, error) {
	r.Body = http.MaxBytesReader(w, r.Body, chat.MaxMessageRequestBytes)

	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		return chat.CreateMessageParams{}, chat.ValidationError{
			StatusCode: http.StatusBadRequest,
			Message:    "invalid Content-Type header",
		}
	}

	switch {
	case mediaType == "" || mediaType == "application/json":
		return decodeJSONMessageRequest(r)
	case mediaType == "multipart/form-data":
		return decodeMultipartMessageRequest(r)
	default:
		return chat.CreateMessageParams{}, chat.ValidationError{
			StatusCode: http.StatusUnsupportedMediaType,
			Message:    "Content-Type must be application/json or multipart/form-data",
		}
	}
}

func decodeJSONMessageRequest(r *http.Request) (chat.CreateMessageParams, error) {
	defer r.Body.Close()

	var body createMessageJSONRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&body); err != nil {
		return chat.CreateMessageParams{}, classifyDecodeError(err)
	}

	return chat.CreateMessageParams{
		Text: coalesceText(body.Text, body.Message),
	}, nil
}

func decodeMultipartMessageRequest(r *http.Request) (chat.CreateMessageParams, error) {
	if err := r.ParseMultipartForm(chat.MaxMessageRequestBytes); err != nil {
		return chat.CreateMessageParams{}, classifyDecodeError(err)
	}
	defer r.MultipartForm.RemoveAll()

	attachments, err := chat.ReadAttachments(r.MultipartForm.File["attachments"])
	if err != nil {
		return chat.CreateMessageParams{}, err
	}

	return chat.CreateMessageParams{
		Text:        coalesceText(r.FormValue("text"), r.FormValue("message")),
		Attachments: attachments,
	}, nil
}

func coalesceText(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}

	return ""
}

func tokenUsageField[T any](value *T, getter func(*T) *int64) *int64 {
	if value == nil {
		return nil
	}

	return getter(value)
}

func writeServiceError(ctx context.Context, w http.ResponseWriter, err error) {
	var validationErr chat.ValidationError
	if errors.As(err, &validationErr) {
		writeJSONError(ctx, w, validationErr.StatusCode, errorCodeForStatus(validationErr.StatusCode), validationErr.Message)
		return
	}

	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		writeJSONError(ctx, w, http.StatusRequestEntityTooLarge, "payload_too_large", "request body exceeds maximum allowed size")
		return
	}

	if errors.Is(err, context.DeadlineExceeded) {
		writeJSONError(ctx, w, http.StatusGatewayTimeout, "request_timeout", "request timed out")
		return
	}

	if errors.Is(err, context.Canceled) {
		writeJSONError(ctx, w, http.StatusGatewayTimeout, "request_canceled", "request was canceled")
		return
	}

	slog.ErrorContext(ctx, "request failed", logFields(ctx, "error", err)...)
	writeJSONError(ctx, w, http.StatusInternalServerError, "internal_error", "internal server error")
}

func classifyDecodeError(err error) error {
	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) {
		return chat.ValidationError{
			StatusCode: http.StatusBadRequest,
			Message:    "request body contains invalid JSON",
		}
	}
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return chat.ValidationError{
			StatusCode: http.StatusBadRequest,
			Message:    "request body contains invalid JSON",
		}
	}

	if errors.Is(err, io.EOF) {
		return chat.ValidationError{
			StatusCode: http.StatusBadRequest,
			Message:    "request body is required",
		}
	}

	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		return chat.ValidationError{
			StatusCode: http.StatusRequestEntityTooLarge,
			Message:    "request body exceeds maximum allowed size",
		}
	}

	if strings.Contains(err.Error(), "http: request body too large") {
		return chat.ValidationError{
			StatusCode: http.StatusRequestEntityTooLarge,
			Message:    "request body exceeds maximum allowed size",
		}
	}

	if strings.Contains(err.Error(), "unknown field") {
		return chat.ValidationError{
			StatusCode: http.StatusBadRequest,
			Message:    err.Error(),
		}
	}

	return chat.ValidationError{
		StatusCode: http.StatusBadRequest,
		Message:    "invalid request body",
	}
}

func writeJSONError(ctx context.Context, w http.ResponseWriter, statusCode int, code, message string) {
	writeJSON(w, statusCode, errorResponse{
		Error:     message,
		Code:      code,
		RequestID: requestIDFromContext(ctx),
	})
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		slog.Error("encode response", "error", err)
	}
}

func writeSSEEvent(w http.ResponseWriter, eventName string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	if _, err := io.WriteString(w, "event: "+eventName+"\n"); err != nil {
		return err
	}
	if _, err := io.WriteString(w, "data: "+string(data)+"\n\n"); err != nil {
		return err
	}

	return nil
}

func (h *Handler) timeoutMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		timeout := h.timeoutForRequest(r)
		if timeout <= 0 {
			next.ServeHTTP(w, r)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		recorder := newBufferingResponseWriter()
		done := make(chan struct{})

		go func() {
			defer close(done)
			next.ServeHTTP(recorder, r.Clone(ctx))
		}()

		select {
		case <-done:
			recorder.CopyTo(w)
		case <-ctx.Done():
			writeJSONError(r.Context(), w, http.StatusGatewayTimeout, "request_timeout", "request timed out")
		}
	})
}

func (h *Handler) timeoutForRequest(r *http.Request) time.Duration {
	if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/stream") {
		return 0
	}
	if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/messages") {
		return h.options.MessageRequestTimeout
	}

	return h.options.RequestTimeout
}

type requestIDContextKey struct{}

type statusRecorder struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func (r *statusRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *statusRecorder) Write(data []byte) (int, error) {
	if r.statusCode == 0 {
		r.statusCode = http.StatusOK
	}

	n, err := r.ResponseWriter.Write(data)
	r.bytesWritten += n
	return n, err
}

func (r *statusRecorder) Flush() {
	flusher, ok := r.ResponseWriter.(http.Flusher)
	if ok {
		flusher.Flush()
	}
}

type bufferingResponseWriter struct {
	header     http.Header
	body       bytes.Buffer
	statusCode int
}

func newBufferingResponseWriter() *bufferingResponseWriter {
	return &bufferingResponseWriter{
		header:     make(http.Header),
		statusCode: http.StatusOK,
	}
}

func (w *bufferingResponseWriter) Header() http.Header {
	return w.header
}

func (w *bufferingResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}

func (w *bufferingResponseWriter) Write(data []byte) (int, error) {
	return w.body.Write(data)
}

func (w *bufferingResponseWriter) CopyTo(target http.ResponseWriter) {
	copyHeader(target.Header(), w.header)
	target.WriteHeader(w.statusCode)
	_, _ = target.Write(w.body.Bytes())
}

func copyHeader(dst, src http.Header) {
	for key, values := range src {
		dst.Del(key)
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func newRequestID() string {
	var data [16]byte
	if _, err := rand.Read(data[:]); err != nil {
		return fmt.Sprintf("req-%d", time.Now().UnixNano())
	}

	return fmt.Sprintf("%x", data[:])
}

func requestIDFromContext(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDContextKey{}).(string)
	return requestID
}

func logFields(ctx context.Context, attrs ...any) []any {
	requestID := requestIDFromContext(ctx)
	if requestID == "" {
		return attrs
	}

	fields := make([]any, 0, len(attrs)+2)
	fields = append(fields, "request_id", requestID)
	fields = append(fields, attrs...)
	return fields
}

func errorCodeForStatus(statusCode int) string {
	switch statusCode {
	case http.StatusBadRequest:
		return "invalid_request"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusConflict:
		return "conflict"
	case http.StatusRequestEntityTooLarge:
		return "payload_too_large"
	case http.StatusUnsupportedMediaType:
		return "unsupported_media_type"
	default:
		return "request_failed"
	}
}
