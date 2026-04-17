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
	"mime/multipart"
	"net/http"
	"net/url"
	"runtime/debug"
	"slices"
	"strings"
	"time"

	"github.com/ivanlin/ulduar/apps/backend/internal/chat"
	"github.com/ivanlin/ulduar/apps/backend/internal/imagegen"
	"github.com/ivanlin/ulduar/apps/backend/internal/presentationgen"
	"github.com/ivanlin/ulduar/apps/backend/internal/repository"
)

type Handler struct {
	chatService            chatService
	imageGenerationService imageGenerationService
	presentationService    presentationGenerationService
	options                HandlerOptions
}

type HandlerOptions struct {
	RequestTimeout                        time.Duration
	MessageRequestTimeout                 time.Duration
	ImageGenerationPollInterval           time.Duration
	ImageGenerationMaxReferenceImageBytes int64
	ImageGenerationService                imageGenerationService
	PresentationGenerationService         presentationGenerationService
	PresentationGenerationPollInterval    time.Duration
}

type chatService interface {
	CreateSession(ctx context.Context) (repository.Session, error)
	GetSession(ctx context.Context, sessionID string) (chat.SessionView, error)
	CreateMessage(ctx context.Context, params chat.CreateMessageParams) (chat.MessageCreation, error)
	StreamRun(ctx context.Context, sessionID, runID string, emit func(chat.RunStreamEvent) error) error
}

type imageGenerationService interface {
	Capabilities() imagegen.Capabilities
	ProviderConfigured() bool
	CreatePendingGeneration(ctx context.Context, params imagegen.CreateGenerationParams) (imagegen.GenerationView, error)
	GetGeneration(ctx context.Context, sessionID, generationID string) (imagegen.GenerationView, error)
	ExecuteGeneration(ctx context.Context, generationID string) error
	GetAssetContent(ctx context.Context, sessionID, generationID, assetID string) (imagegen.AssetContent, error)
	GetImageContent(ctx context.Context, sessionID, generationID, imageID string) (imagegen.AssetContent, error)
}

type presentationGenerationService interface {
	Capabilities() presentationgen.Capabilities
	ProviderConfigured() bool
	CreatePendingGeneration(ctx context.Context, params presentationgen.CreateGenerationParams) (presentationgen.GenerationView, error)
	GetGeneration(ctx context.Context, sessionID, generationID string) (presentationgen.GenerationView, error)
	ExecuteGeneration(ctx context.Context, generationID string) error
	GetAssetContent(ctx context.Context, sessionID, generationID, assetID string) (presentationgen.AssetContent, error)
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

type imageGenerationCapabilitiesResponse struct {
	Modes              []string                    `json:"modes"`
	Resolutions        []imageGenerationResolution `json:"resolutions"`
	MaxReferenceImages int                         `json:"maxReferenceImages"`
	OutputImageCount   int                         `json:"outputImageCount"`
	ProviderName       string                      `json:"providerName,omitempty"`
}

type imageGenerationResolution struct {
	Key    string `json:"key"`
	Width  int64  `json:"width"`
	Height int64  `json:"height"`
}

type createImageGenerationJSONRequest struct {
	Mode       string `json:"mode"`
	Prompt     string `json:"prompt"`
	Resolution string `json:"resolution"`
}

type createImageGenerationResponse struct {
	GenerationID string    `json:"generationId"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"createdAt"`
}

type presentationGenerationCapabilitiesResponse struct {
	InputMediaTypes []string `json:"inputMediaTypes"`
	OutputMediaType string   `json:"outputMediaType"`
	ProviderName    string   `json:"providerName,omitempty"`
}

type createPresentationGenerationJSONRequest struct {
	Prompt string `json:"prompt"`
}

type createPresentationGenerationResponse struct {
	GenerationID string    `json:"generationId"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"createdAt"`
}

type presentationGenerationResponse struct {
	GenerationID  string                              `json:"generationId"`
	SessionID     string                              `json:"sessionId"`
	Status        string                              `json:"status"`
	Prompt        string                              `json:"prompt"`
	DialectJSON   json.RawMessage                     `json:"dialectJson,omitempty"`
	ProviderName  string                              `json:"providerName,omitempty"`
	ProviderModel string                              `json:"providerModel,omitempty"`
	ErrorCode     string                              `json:"errorCode,omitempty"`
	ErrorMessage  string                              `json:"errorMessage,omitempty"`
	CreatedAt     time.Time                           `json:"createdAt"`
	CompletedAt   *time.Time                          `json:"completedAt,omitempty"`
	InputAssets   []presentationGenerationAssetResult `json:"inputAssets"`
	OutputAssets  []presentationGenerationAssetResult `json:"outputAssets"`
}

type presentationGenerationAssetResult struct {
	AssetID    string    `json:"assetId"`
	Filename   string    `json:"filename"`
	MediaType  string    `json:"mediaType"`
	SizeBytes  int64     `json:"sizeBytes"`
	SHA256     string    `json:"sha256"`
	CreatedAt  time.Time `json:"createdAt"`
	ContentURL string    `json:"contentUrl,omitempty"`
}

type imageGenerationResponse struct {
	GenerationID     string                       `json:"generationId"`
	SessionID        string                       `json:"sessionId"`
	Mode             string                       `json:"mode"`
	Status           string                       `json:"status"`
	Prompt           string                       `json:"prompt"`
	Resolution       imageGenerationResolution    `json:"resolution"`
	OutputImageCount int64                        `json:"outputImageCount"`
	ProviderName     string                       `json:"providerName,omitempty"`
	ProviderModel    string                       `json:"providerModel,omitempty"`
	ErrorCode        string                       `json:"errorCode,omitempty"`
	ErrorMessage     string                       `json:"errorMessage,omitempty"`
	CreatedAt        time.Time                    `json:"createdAt"`
	CompletedAt      *time.Time                   `json:"completedAt,omitempty"`
	InputAssets      []imageGenerationAssetResult `json:"inputAssets"`
	OutputAssets     []imageGenerationAssetResult `json:"outputAssets"`
}

type imageGenerationAssetResult struct {
	AssetID    string    `json:"assetId"`
	Filename   string    `json:"filename"`
	MediaType  string    `json:"mediaType"`
	SizeBytes  int64     `json:"sizeBytes"`
	SHA256     string    `json:"sha256"`
	Width      *int64    `json:"width,omitempty"`
	Height     *int64    `json:"height,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
	ContentURL string    `json:"contentUrl,omitempty"`
}

const imageGenerationMultipartMemoryBytes int64 = 1 << 20

const (
	imageGenerationAssetContentPathSegmentCount  = 9
	imageGenerationAssetContentIndexAPI          = 0
	imageGenerationAssetContentIndexVersion      = 1
	imageGenerationAssetContentIndexSessions     = 2
	imageGenerationAssetContentIndexSessionID    = 3
	imageGenerationAssetContentIndexGenerations  = 4
	imageGenerationAssetContentIndexGenerationID = 5
	imageGenerationAssetContentIndexAssets       = 6
	imageGenerationAssetContentIndexAssetID      = 7
	imageGenerationAssetContentIndexContent      = 8
)

func NewHandler(chatService chatService, options ...HandlerOptions) http.Handler {
	handlerOptions := HandlerOptions{
		RequestTimeout:                        15 * time.Second,
		MessageRequestTimeout:                 90 * time.Second,
		ImageGenerationPollInterval:           time.Second,
		ImageGenerationMaxReferenceImageBytes: imagegen.DefaultMaxReferenceImageBytes,
		PresentationGenerationPollInterval:    time.Second,
	}
	if len(options) > 0 {
		provided := options[0]
		if provided.RequestTimeout > 0 {
			handlerOptions.RequestTimeout = provided.RequestTimeout
		}
		if provided.MessageRequestTimeout > 0 {
			handlerOptions.MessageRequestTimeout = provided.MessageRequestTimeout
		}
		if provided.ImageGenerationPollInterval > 0 {
			handlerOptions.ImageGenerationPollInterval = provided.ImageGenerationPollInterval
		}
		if provided.ImageGenerationMaxReferenceImageBytes > 0 {
			handlerOptions.ImageGenerationMaxReferenceImageBytes = provided.ImageGenerationMaxReferenceImageBytes
		}
		if provided.ImageGenerationService != nil {
			handlerOptions.ImageGenerationService = provided.ImageGenerationService
		}
		if provided.PresentationGenerationService != nil {
			handlerOptions.PresentationGenerationService = provided.PresentationGenerationService
		}
		if provided.PresentationGenerationPollInterval > 0 {
			handlerOptions.PresentationGenerationPollInterval = provided.PresentationGenerationPollInterval
		}
	}

	handler := &Handler{
		chatService:            chatService,
		imageGenerationService: handlerOptions.ImageGenerationService,
		presentationService:    handlerOptions.PresentationGenerationService,
		options:                handlerOptions,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthHandler)
	mux.HandleFunc("POST /api/v1/sessions", handler.createSessionHandler)
	mux.HandleFunc("GET /api/v1/sessions/{sessionId}", handler.getSessionHandler)
	mux.HandleFunc("POST /api/v1/sessions/{sessionId}/messages", handler.createMessageHandler)
	mux.HandleFunc("GET /api/v1/sessions/{sessionId}/runs/{runId}/stream", handler.streamRunHandler)
	mux.HandleFunc("GET /api/v1/image-generations/capabilities", handler.imageGenerationCapabilitiesHandler)
	mux.HandleFunc("GET /api/v1/presentation-generations/capabilities", handler.presentationGenerationCapabilitiesHandler)
	mux.HandleFunc("POST /api/v1/sessions/{sessionId}/image-generations", handler.createImageGenerationHandler)
	mux.HandleFunc("POST /api/v1/sessions/{sessionId}/presentation-generations", handler.createPresentationGenerationHandler)
	mux.HandleFunc("GET /api/v1/sessions/{sessionId}/image-generations/{generationId}", handler.getImageGenerationHandler)
	mux.HandleFunc("GET /api/v1/sessions/{sessionId}/presentation-generations/{generationId}", handler.getPresentationGenerationHandler)
	mux.HandleFunc("GET /api/v1/sessions/{sessionId}/image-generations/{generationId}/stream", handler.streamImageGenerationHandler)
	mux.HandleFunc("GET /api/v1/sessions/{sessionId}/presentation-generations/{generationId}/stream", handler.streamPresentationGenerationHandler)
	mux.HandleFunc("GET /api/v1/sessions/{sessionId}/image-generations/{generationId}/assets/{assetId}/content", handler.getImageGenerationAssetContentHandler)
	mux.HandleFunc("GET /api/v1/sessions/{sessionId}/presentation-generations/{generationId}/assets/{assetId}/content", handler.getPresentationGenerationAssetContentHandler)
	mux.HandleFunc("GET /api/v1/sessions/{sessionId}/image-generations/{generationId}/images/{imageId}/content", handler.getImageGenerationImageContentHandler)
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

func (h *Handler) imageGenerationCapabilitiesHandler(w http.ResponseWriter, r *http.Request) {
	if h.imageGenerationService == nil {
		writeImageGenerationUnavailable(r.Context(), w)
		return
	}
	if !h.imageGenerationService.ProviderConfigured() {
		writeImageGenerationUnavailable(r.Context(), w)
		return
	}

	writeJSON(w, http.StatusOK, mapImageGenerationCapabilities(h.imageGenerationService.Capabilities()))
}

func (h *Handler) presentationGenerationCapabilitiesHandler(w http.ResponseWriter, r *http.Request) {
	if h.presentationService == nil {
		writePresentationGenerationUnavailable(r.Context(), w)
		return
	}
	if !h.presentationService.ProviderConfigured() {
		writePresentationGenerationUnavailable(r.Context(), w)
		return
	}

	writeJSON(w, http.StatusOK, mapPresentationGenerationCapabilities(h.presentationService.Capabilities()))
}

func (h *Handler) createImageGenerationHandler(w http.ResponseWriter, r *http.Request) {
	if h.imageGenerationService == nil {
		writeImageGenerationUnavailable(r.Context(), w)
		return
	}
	if !h.imageGenerationService.ProviderConfigured() {
		writeImageGenerationUnavailable(r.Context(), w)
		return
	}

	params, err := decodeCreateImageGenerationRequest(w, r, h.options.ImageGenerationMaxReferenceImageBytes)
	if err != nil {
		writeServiceError(r.Context(), w, err)
		return
	}
	params.SessionID = r.PathValue("sessionId")

	view, err := h.imageGenerationService.CreatePendingGeneration(r.Context(), params)
	if err != nil {
		writeServiceError(r.Context(), w, err)
		return
	}

	writeJSON(w, http.StatusAccepted, createImageGenerationResponse{
		GenerationID: view.Generation.ID,
		Status:       string(view.Generation.Status),
		CreatedAt:    view.Generation.CreatedAt,
	})
}

func (h *Handler) createPresentationGenerationHandler(w http.ResponseWriter, r *http.Request) {
	if h.presentationService == nil {
		writePresentationGenerationUnavailable(r.Context(), w)
		return
	}
	if !h.presentationService.ProviderConfigured() {
		writePresentationGenerationUnavailable(r.Context(), w)
		return
	}

	params, err := decodeCreatePresentationGenerationRequest(w, r)
	if err != nil {
		writeServiceError(r.Context(), w, err)
		return
	}
	params.SessionID = r.PathValue("sessionId")

	view, err := h.presentationService.CreatePendingGeneration(r.Context(), params)
	if err != nil {
		writeServiceError(r.Context(), w, err)
		return
	}

	writeJSON(w, http.StatusAccepted, createPresentationGenerationResponse{
		GenerationID: view.Generation.ID,
		Status:       string(view.Generation.Status),
		CreatedAt:    view.Generation.CreatedAt,
	})
}

func (h *Handler) getImageGenerationHandler(w http.ResponseWriter, r *http.Request) {
	if h.imageGenerationService == nil {
		writeImageGenerationUnavailable(r.Context(), w)
		return
	}

	view, err := h.imageGenerationService.GetGeneration(r.Context(), r.PathValue("sessionId"), r.PathValue("generationId"))
	if err != nil {
		writeServiceError(r.Context(), w, err)
		return
	}

	writeJSON(w, http.StatusOK, mapImageGenerationResponse(view))
}

func (h *Handler) getPresentationGenerationHandler(w http.ResponseWriter, r *http.Request) {
	if h.presentationService == nil {
		writePresentationGenerationUnavailable(r.Context(), w)
		return
	}

	view, err := h.presentationService.GetGeneration(r.Context(), r.PathValue("sessionId"), r.PathValue("generationId"))
	if err != nil {
		writeServiceError(r.Context(), w, err)
		return
	}

	writeJSON(w, http.StatusOK, mapPresentationGenerationResponse(view))
}

func (h *Handler) getImageGenerationAssetContentHandler(w http.ResponseWriter, r *http.Request) {
	if h.imageGenerationService == nil {
		writeImageGenerationUnavailable(r.Context(), w)
		return
	}

	content, err := h.imageGenerationService.GetAssetContent(
		r.Context(),
		r.PathValue("sessionId"),
		r.PathValue("generationId"),
		r.PathValue("assetId"),
	)
	if err != nil {
		writeServiceError(r.Context(), w, err)
		return
	}

	writeAssetContent(r.Context(), w, content, "")
}

func (h *Handler) getPresentationGenerationAssetContentHandler(w http.ResponseWriter, r *http.Request) {
	if h.presentationService == nil {
		writePresentationGenerationUnavailable(r.Context(), w)
		return
	}

	content, err := h.presentationService.GetAssetContent(
		r.Context(),
		r.PathValue("sessionId"),
		r.PathValue("generationId"),
		r.PathValue("assetId"),
	)
	if err != nil {
		writeServiceError(r.Context(), w, err)
		return
	}

	writePresentationAssetContent(r.Context(), w, content, "")
}

func (h *Handler) getImageGenerationImageContentHandler(w http.ResponseWriter, r *http.Request) {
	if h.imageGenerationService == nil {
		writeImageGenerationUnavailable(r.Context(), w)
		return
	}

	content, err := h.imageGenerationService.GetImageContent(
		r.Context(),
		r.PathValue("sessionId"),
		r.PathValue("generationId"),
		r.PathValue("imageId"),
	)
	if err != nil {
		writeServiceError(r.Context(), w, err)
		return
	}

	writeAssetContent(r.Context(), w, content, "public, max-age=31536000, immutable")
}

func writeAssetContent(ctx context.Context, w http.ResponseWriter, content imagegen.AssetContent, cacheControl string) {
	writeBinaryContent(ctx, w, content.Filename, content.MediaType, content.Data, cacheControl, "image generation")
}

func writePresentationAssetContent(ctx context.Context, w http.ResponseWriter, content presentationgen.AssetContent, cacheControl string) {
	writeBinaryContent(ctx, w, content.Filename, content.MediaType, content.Data, cacheControl, "presentation generation")
}

func writeBinaryContent(ctx context.Context, w http.ResponseWriter, filename, mediaType string, data []byte, cacheControl string, logLabel string) {
	if mediaType != "" {
		w.Header().Set("Content-Type", mediaType)
	}
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	if cacheControl != "" {
		w.Header().Set("Cache-Control", cacheControl)
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if filename != "" {
		w.Header().Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": filename}))
	}
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		slog.ErrorContext(ctx, "write "+logLabel+" content", logFields(ctx, "error", err)...)
	}
}

func (h *Handler) streamImageGenerationHandler(w http.ResponseWriter, r *http.Request) {
	if h.imageGenerationService == nil {
		writeImageGenerationUnavailable(r.Context(), w)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSONError(r.Context(), w, http.StatusInternalServerError, "streaming_unsupported", "streaming is not supported")
		return
	}

	sessionID := r.PathValue("sessionId")
	generationID := r.PathValue("generationId")
	streamStarted := false
	emit := func(eventName string, view imagegen.GenerationView) error {
		if err := writeSSEEvent(w, eventName, mapImageGenerationResponse(view)); err != nil {
			return err
		}
		streamStarted = true
		flusher.Flush()
		return nil
	}

	view, err := h.imageGenerationService.GetGeneration(r.Context(), sessionID, generationID)
	if err != nil {
		writeServiceError(r.Context(), w, err)
		return
	}

	if _, done := imageGenerationTerminalEvent(view.Generation.Status); !done && !h.imageGenerationService.ProviderConfigured() {
		writeImageGenerationUnavailable(r.Context(), w)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	if terminalEvent, done := imageGenerationTerminalEvent(view.Generation.Status); done {
		if err := emit(terminalEvent, view); err != nil {
			slog.ErrorContext(r.Context(), "write image generation terminal event", logFields(r.Context(), "generation_id", generationID, "error", err)...)
		}
		return
	}

	runningEmitted := false
	if view.Generation.Status == imagegen.StatusPending {
		if err := emit("image_generation.started", view); err != nil {
			slog.ErrorContext(r.Context(), "write image generation started event", logFields(r.Context(), "generation_id", generationID, "error", err)...)
			return
		}
	} else {
		if err := emit("image_generation.running", view); err != nil {
			slog.ErrorContext(r.Context(), "write image generation running event", logFields(r.Context(), "generation_id", generationID, "error", err)...)
			return
		}
		runningEmitted = true
	}

	ticker := time.NewTicker(h.options.ImageGenerationPollInterval)
	defer ticker.Stop()

	for {
		execErr := h.imageGenerationService.ExecuteGeneration(r.Context(), generationID)
		view, err = h.imageGenerationService.GetGeneration(r.Context(), sessionID, generationID)
		if err != nil {
			if !streamStarted {
				writeServiceError(r.Context(), w, err)
				return
			}
			slog.ErrorContext(r.Context(), "refresh image generation stream state", logFields(r.Context(), "generation_id", generationID, "error", err)...)
			return
		}

		if view.Generation.Status == imagegen.StatusFailed {
			if err := emit("image_generation.failed", view); err != nil {
				slog.ErrorContext(r.Context(), "write image generation failed event", logFields(r.Context(), "generation_id", generationID, "error", err)...)
			}
			return
		}
		if execErr != nil {
			slog.ErrorContext(r.Context(), "image generation execution failed", logFields(r.Context(), "generation_id", generationID, "error", execErr)...)
			return
		}

		switch view.Generation.Status {
		case imagegen.StatusPending:
		case imagegen.StatusRunning:
			if !runningEmitted {
				if err := emit("image_generation.running", view); err != nil {
					slog.ErrorContext(r.Context(), "write image generation running event", logFields(r.Context(), "generation_id", generationID, "error", err)...)
					return
				}
				runningEmitted = true
			}
		case imagegen.StatusCompleted:
			if err := emit("image_generation.completed", view); err != nil {
				slog.ErrorContext(r.Context(), "write image generation completed event", logFields(r.Context(), "generation_id", generationID, "error", err)...)
			}
			return
		default:
			err := fmt.Errorf("unsupported image generation status %q", view.Generation.Status)
			if !streamStarted {
				writeServiceError(r.Context(), w, err)
				return
			}
			slog.ErrorContext(r.Context(), "image generation stream failed", logFields(r.Context(), "generation_id", generationID, "error", err)...)
			return
		}

		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
		}
	}
}

func (h *Handler) streamPresentationGenerationHandler(w http.ResponseWriter, r *http.Request) {
	if h.presentationService == nil {
		writePresentationGenerationUnavailable(r.Context(), w)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSONError(r.Context(), w, http.StatusInternalServerError, "streaming_unsupported", "streaming is not supported")
		return
	}

	sessionID := r.PathValue("sessionId")
	generationID := r.PathValue("generationId")
	streamStarted := false
	emit := func(eventName string, view presentationgen.GenerationView) error {
		if err := writeSSEEvent(w, eventName, mapPresentationGenerationResponse(view)); err != nil {
			return err
		}
		streamStarted = true
		flusher.Flush()
		return nil
	}

	view, err := h.presentationService.GetGeneration(r.Context(), sessionID, generationID)
	if err != nil {
		writeServiceError(r.Context(), w, err)
		return
	}

	if _, done := presentationGenerationTerminalEvent(view.Generation.Status); !done && !h.presentationService.ProviderConfigured() {
		writePresentationGenerationUnavailable(r.Context(), w)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	if terminalEvent, done := presentationGenerationTerminalEvent(view.Generation.Status); done {
		if err := emit(terminalEvent, view); err != nil {
			slog.ErrorContext(r.Context(), "write presentation generation terminal event", logFields(r.Context(), "generation_id", generationID, "error", err)...)
		}
		return
	}

	runningEmitted := false
	if view.Generation.Status == presentationgen.StatusPending {
		if err := emit("presentation_generation.started", view); err != nil {
			slog.ErrorContext(r.Context(), "write presentation generation started event", logFields(r.Context(), "generation_id", generationID, "error", err)...)
			return
		}
	} else {
		if err := emit("presentation_generation.running", view); err != nil {
			slog.ErrorContext(r.Context(), "write presentation generation running event", logFields(r.Context(), "generation_id", generationID, "error", err)...)
			return
		}
		runningEmitted = true
	}

	ticker := time.NewTicker(h.options.PresentationGenerationPollInterval)
	defer ticker.Stop()

	for {
		execErr := h.presentationService.ExecuteGeneration(r.Context(), generationID)
		view, err = h.presentationService.GetGeneration(r.Context(), sessionID, generationID)
		if err != nil {
			if !streamStarted {
				writeServiceError(r.Context(), w, err)
				return
			}
			slog.ErrorContext(r.Context(), "refresh presentation generation stream state", logFields(r.Context(), "generation_id", generationID, "error", err)...)
			return
		}

		if view.Generation.Status == presentationgen.StatusFailed {
			if err := emit("presentation_generation.failed", view); err != nil {
				slog.ErrorContext(r.Context(), "write presentation generation failed event", logFields(r.Context(), "generation_id", generationID, "error", err)...)
			}
			return
		}
		if execErr != nil {
			slog.ErrorContext(r.Context(), "presentation generation execution failed", logFields(r.Context(), "generation_id", generationID, "error", execErr)...)
			return
		}

		switch view.Generation.Status {
		case presentationgen.StatusPending:
		case presentationgen.StatusRunning:
			if !runningEmitted {
				if err := emit("presentation_generation.running", view); err != nil {
					slog.ErrorContext(r.Context(), "write presentation generation running event", logFields(r.Context(), "generation_id", generationID, "error", err)...)
					return
				}
				runningEmitted = true
			}
		case presentationgen.StatusCompleted:
			if err := emit("presentation_generation.completed", view); err != nil {
				slog.ErrorContext(r.Context(), "write presentation generation completed event", logFields(r.Context(), "generation_id", generationID, "error", err)...)
			}
			return
		default:
			err := fmt.Errorf("unsupported presentation generation status %q", view.Generation.Status)
			if !streamStarted {
				writeServiceError(r.Context(), w, err)
				return
			}
			slog.ErrorContext(r.Context(), "presentation generation stream failed", logFields(r.Context(), "generation_id", generationID, "error", err)...)
			return
		}

		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
		}
	}
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
			if event.ToolName != "" {
				payload["toolName"] = event.ToolName
			}
			if event.ToolPhase != "" {
				payload["toolPhase"] = event.ToolPhase
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

func decodeCreateImageGenerationRequest(w http.ResponseWriter, r *http.Request, maxReferenceImageBytes int64) (imagegen.CreateGenerationParams, error) {
	maxRequestBytes := maxReferenceImageBytes * int64(imagegen.MaxReferenceImages)
	if maxRequestBytes <= 0 {
		maxRequestBytes = imagegen.DefaultMaxReferenceImageBytes * int64(imagegen.MaxReferenceImages)
	}
	// Allow a small buffer for multipart boundaries and form field metadata in
	// addition to the raw reference image byte budget.
	maxRequestBytes += 1 << 20
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)

	mediaType, err := optionalRequestMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		return imagegen.CreateGenerationParams{}, imagegen.ValidationError{
			StatusCode: http.StatusBadRequest,
			Message:    "invalid Content-Type header",
		}
	}

	switch {
	case mediaType == "" || mediaType == "application/json":
		// Missing Content-Type defaults to JSON to match the existing chat create
		// handler behavior and keep simple POST clients working without headers.
		return decodeCreateImageGenerationJSONRequest(r)
	case mediaType == "multipart/form-data":
		return decodeCreateImageGenerationMultipartRequest(r, maxRequestBytes)
	default:
		return imagegen.CreateGenerationParams{}, imagegen.ValidationError{
			StatusCode: http.StatusUnsupportedMediaType,
			Message:    "Content-Type must be application/json or multipart/form-data",
		}
	}
}

func decodeCreatePresentationGenerationRequest(w http.ResponseWriter, r *http.Request) (presentationgen.CreateGenerationParams, error) {
	r.Body = http.MaxBytesReader(w, r.Body, chat.MaxMessageRequestBytes)

	mediaType, err := optionalRequestMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		return presentationgen.CreateGenerationParams{}, chat.ValidationError{
			StatusCode: http.StatusBadRequest,
			Message:    "invalid Content-Type header",
		}
	}

	switch {
	case mediaType == "" || mediaType == "application/json":
		return decodeCreatePresentationGenerationJSONRequest(r)
	case mediaType == "multipart/form-data":
		return decodeCreatePresentationGenerationMultipartRequest(r)
	default:
		return presentationgen.CreateGenerationParams{}, chat.ValidationError{
			StatusCode: http.StatusUnsupportedMediaType,
			Message:    "Content-Type must be application/json or multipart/form-data",
		}
	}
}

// optionalRequestMediaType parses Content-Type when present and returns an empty
// media type for missing headers so callers can apply their own defaults.
func optionalRequestMediaType(header string) (string, error) {
	header = strings.TrimSpace(header)
	if header == "" {
		return "", nil
	}

	mediaType, _, err := mime.ParseMediaType(header)
	if err != nil {
		return "", err
	}

	return mediaType, nil
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

func decodeCreateImageGenerationJSONRequest(r *http.Request) (imagegen.CreateGenerationParams, error) {
	defer r.Body.Close()

	var body createImageGenerationJSONRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&body); err != nil {
		return imagegen.CreateGenerationParams{}, classifyDecodeError(err)
	}

	mode := imagegen.Mode(strings.TrimSpace(body.Mode))
	if mode == imagegen.ModeImageEdit {
		return imagegen.CreateGenerationParams{}, imagegen.ValidationError{
			StatusCode: http.StatusBadRequest,
			Message:    "image_edit requests must use multipart/form-data",
		}
	}

	return imagegen.CreateGenerationParams{
		Mode:          mode,
		Prompt:        body.Prompt,
		ResolutionKey: body.Resolution,
	}, nil
}

func decodeCreateImageGenerationMultipartRequest(r *http.Request, maxRequestBytes int64) (imagegen.CreateGenerationParams, error) {
	multipartMemoryLimit := min(maxRequestBytes, imageGenerationMultipartMemoryBytes)
	if err := r.ParseMultipartForm(multipartMemoryLimit); err != nil {
		return imagegen.CreateGenerationParams{}, classifyDecodeError(err)
	}
	defer r.MultipartForm.RemoveAll()

	referenceImages, err := readReferenceImages(r.MultipartForm.File["referenceImages"], r.MultipartForm.File["referenceImages[]"])
	if err != nil {
		return imagegen.CreateGenerationParams{}, err
	}

	return imagegen.CreateGenerationParams{
		Mode:            imagegen.Mode(strings.TrimSpace(r.FormValue("mode"))),
		Prompt:          r.FormValue("prompt"),
		ResolutionKey:   r.FormValue("resolution"),
		ReferenceImages: referenceImages,
	}, nil
}

func decodeCreatePresentationGenerationJSONRequest(r *http.Request) (presentationgen.CreateGenerationParams, error) {
	defer r.Body.Close()

	var body createPresentationGenerationJSONRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&body); err != nil {
		return presentationgen.CreateGenerationParams{}, classifyDecodeError(err)
	}

	return presentationgen.CreateGenerationParams{
		Prompt: body.Prompt,
	}, nil
}

func decodeCreatePresentationGenerationMultipartRequest(r *http.Request) (presentationgen.CreateGenerationParams, error) {
	if err := r.ParseMultipartForm(chat.MaxMessageRequestBytes); err != nil {
		return presentationgen.CreateGenerationParams{}, classifyDecodeError(err)
	}
	defer r.MultipartForm.RemoveAll()

	uploads := combineMultipartFileHeaders(r.MultipartForm.File["attachments"], r.MultipartForm.File["attachments[]"])
	inputs, err := readPresentationInputUploads(uploads)
	if err != nil {
		return presentationgen.CreateGenerationParams{}, err
	}

	return presentationgen.CreateGenerationParams{
		Prompt:      r.FormValue("prompt"),
		Attachments: inputs,
	}, nil
}

func readPresentationInputUploads(headers []*multipart.FileHeader) ([]presentationgen.InputAssetUpload, error) {
	attachments, err := chat.ReadAttachments(headers)
	if err != nil {
		return nil, err
	}

	supportedMediaTypes := presentationgen.SupportedInputMediaTypes()
	inputs := make([]presentationgen.InputAssetUpload, 0, len(attachments))
	for _, attachment := range attachments {
		if !slices.Contains(supportedMediaTypes, attachment.MediaType) {
			return nil, presentationgen.ValidationError{
				StatusCode: http.StatusUnsupportedMediaType,
				Message:    fmt.Sprintf("attachment %q has unsupported media type %q", attachment.Filename, attachment.MediaType),
			}
		}

		inputs = append(inputs, presentationgen.InputAssetUpload{
			Filename: attachment.Filename,
			Data:     attachment.Data,
		})
	}

	return inputs, nil
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

func readReferenceImages(fileSets ...[]*multipart.FileHeader) ([]imagegen.InputAssetUpload, error) {
	total := 0
	for _, set := range fileSets {
		total += len(set)
	}

	uploads := make([]imagegen.InputAssetUpload, 0, total)
	for _, fileHeaders := range fileSets {
		for _, header := range fileHeaders {
			file, err := header.Open()
			if err != nil {
				return nil, err
			}
			data, readErr := io.ReadAll(file)
			closeErr := file.Close()
			if readErr != nil {
				return nil, readErr
			}
			if closeErr != nil {
				return nil, closeErr
			}

			uploads = append(uploads, imagegen.InputAssetUpload{
				Filename: header.Filename,
				Data:     data,
			})
		}
	}

	return uploads, nil
}

func combineMultipartFileHeaders(fileSets ...[]*multipart.FileHeader) []*multipart.FileHeader {
	total := 0
	for _, set := range fileSets {
		total += len(set)
	}

	headers := make([]*multipart.FileHeader, 0, total)
	for _, set := range fileSets {
		headers = append(headers, set...)
	}

	return headers
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

func mapImageGenerationCapabilities(capabilities imagegen.Capabilities) imageGenerationCapabilitiesResponse {
	modes := make([]string, 0, len(capabilities.Modes))
	for _, mode := range capabilities.Modes {
		modes = append(modes, string(mode))
	}

	return imageGenerationCapabilitiesResponse{
		Modes:              modes,
		Resolutions:        mapImageGenerationResolutions(capabilities.Resolutions),
		MaxReferenceImages: capabilities.MaxReferenceImages,
		OutputImageCount:   capabilities.OutputImageCount,
		ProviderName:       capabilities.ProviderName,
	}
}

func mapPresentationGenerationCapabilities(capabilities presentationgen.Capabilities) presentationGenerationCapabilitiesResponse {
	return presentationGenerationCapabilitiesResponse{
		InputMediaTypes: slices.Clone(capabilities.InputMediaTypes),
		OutputMediaType: capabilities.OutputMediaType,
		ProviderName:    capabilities.ProviderName,
	}
}

func mapImageGenerationResponse(view imagegen.GenerationView) imageGenerationResponse {
	inputAssets := make([]imageGenerationAssetResult, 0, len(view.Assets))
	outputAssets := make([]imageGenerationAssetResult, 0, len(view.Assets))
	for _, asset := range view.Assets {
		item := imageGenerationAssetResult{
			AssetID:   asset.ID,
			Filename:  asset.Filename,
			MediaType: asset.MediaType,
			SizeBytes: asset.SizeBytes,
			SHA256:    asset.SHA256,
			Width:     asset.Width,
			Height:    asset.Height,
			CreatedAt: asset.CreatedAt,
		}
		if asset.Role == imagegen.AssetRoleOutput {
			item.ContentURL = imageGenerationImageContentURL(view.Generation.SessionID, view.Generation.ID, asset.ID)
			outputAssets = append(outputAssets, item)
			continue
		}
		inputAssets = append(inputAssets, item)
	}

	return imageGenerationResponse{
		GenerationID:     view.Generation.ID,
		SessionID:        view.Generation.SessionID,
		Mode:             string(view.Generation.Mode),
		Status:           string(view.Generation.Status),
		Prompt:           view.Generation.Prompt,
		Resolution:       mapImageGenerationResolution(view.Generation.Resolution),
		OutputImageCount: view.Generation.OutputImageCount,
		ProviderName:     view.Generation.ProviderName,
		ProviderModel:    view.Generation.ProviderModel,
		ErrorCode:        view.Generation.ErrorCode,
		ErrorMessage:     view.Generation.ErrorMessage,
		CreatedAt:        view.Generation.CreatedAt,
		CompletedAt:      view.Generation.CompletedAt,
		InputAssets:      inputAssets,
		OutputAssets:     outputAssets,
	}
}

func mapPresentationGenerationResponse(view presentationgen.GenerationView) presentationGenerationResponse {
	inputAssets := make([]presentationGenerationAssetResult, 0, len(view.Assets))
	outputAssets := make([]presentationGenerationAssetResult, 0, len(view.Assets))
	for _, asset := range view.Assets {
		item := presentationGenerationAssetResult{
			AssetID:   asset.ID,
			Filename:  asset.Filename,
			MediaType: asset.MediaType,
			SizeBytes: asset.SizeBytes,
			SHA256:    asset.SHA256,
			CreatedAt: asset.CreatedAt,
		}
		if asset.Role == presentationgen.AssetRoleOutput {
			item.ContentURL = presentationGenerationAssetContentURL(view.Generation.SessionID, view.Generation.ID, asset.ID)
			outputAssets = append(outputAssets, item)
			continue
		}
		inputAssets = append(inputAssets, item)
	}

	var dialectJSON json.RawMessage
	if len(view.Generation.DialectJSON) > 0 {
		dialectJSON = json.RawMessage(view.Generation.DialectJSON)
	}

	return presentationGenerationResponse{
		GenerationID:  view.Generation.ID,
		SessionID:     view.Generation.SessionID,
		Status:        string(view.Generation.Status),
		Prompt:        view.Generation.Prompt,
		DialectJSON:   dialectJSON,
		ProviderName:  view.Generation.ProviderName,
		ProviderModel: view.Generation.ProviderModel,
		ErrorCode:     view.Generation.ErrorCode,
		ErrorMessage:  view.Generation.ErrorMessage,
		CreatedAt:     view.Generation.CreatedAt,
		CompletedAt:   view.Generation.CompletedAt,
		InputAssets:   inputAssets,
		OutputAssets:  outputAssets,
	}
}

func mapImageGenerationResolutions(resolutions []imagegen.Resolution) []imageGenerationResolution {
	mapped := make([]imageGenerationResolution, 0, len(resolutions))
	for _, resolution := range resolutions {
		mapped = append(mapped, mapImageGenerationResolution(resolution))
	}
	return mapped
}

func mapImageGenerationResolution(resolution imagegen.Resolution) imageGenerationResolution {
	return imageGenerationResolution{
		Key:    resolution.Key,
		Width:  resolution.Width,
		Height: resolution.Height,
	}
}

func imageGenerationImageContentURL(sessionID, generationID, imageID string) string {
	return "/api/v1/sessions/" +
		url.PathEscape(sessionID) +
		"/image-generations/" +
		url.PathEscape(generationID) +
		"/images/" +
		url.PathEscape(imageID) +
		"/content"
}

func presentationGenerationAssetContentURL(sessionID, generationID, assetID string) string {
	return "/api/v1/sessions/" +
		url.PathEscape(sessionID) +
		"/presentation-generations/" +
		url.PathEscape(generationID) +
		"/assets/" +
		url.PathEscape(assetID) +
		"/content"
}

func imageGenerationTerminalEvent(status imagegen.Status) (string, bool) {
	switch status {
	case imagegen.StatusCompleted:
		return "image_generation.completed", true
	case imagegen.StatusFailed:
		return "image_generation.failed", true
	default:
		return "", false
	}
}

func presentationGenerationTerminalEvent(status presentationgen.Status) (string, bool) {
	switch status {
	case presentationgen.StatusCompleted:
		return "presentation_generation.completed", true
	case presentationgen.StatusFailed:
		return "presentation_generation.failed", true
	default:
		return "", false
	}
}

func writeImageGenerationUnavailable(ctx context.Context, w http.ResponseWriter) {
	writeJSONError(ctx, w, http.StatusServiceUnavailable, "service_unavailable", "image generation is not configured")
}

func writePresentationGenerationUnavailable(ctx context.Context, w http.ResponseWriter) {
	writeJSONError(ctx, w, http.StatusServiceUnavailable, "service_unavailable", "presentation generation is not configured")
}

func writeServiceError(ctx context.Context, w http.ResponseWriter, err error) {
	var validationErr chat.ValidationError
	if errors.As(err, &validationErr) {
		writeJSONError(ctx, w, validationErr.StatusCode, errorCodeForStatus(validationErr.StatusCode), validationErr.Message)
		return
	}

	var imageValidationErr imagegen.ValidationError
	if errors.As(err, &imageValidationErr) {
		writeJSONError(ctx, w, imageValidationErr.StatusCode, errorCodeForStatus(imageValidationErr.StatusCode), imageValidationErr.Message)
		return
	}

	var presentationValidationErr presentationgen.ValidationError
	if errors.As(err, &presentationValidationErr) {
		writeJSONError(ctx, w, presentationValidationErr.StatusCode, errorCodeForStatus(presentationValidationErr.StatusCode), presentationValidationErr.Message)
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

		if shouldBypassTimeoutBuffering(r) {
			next.ServeHTTP(w, r.Clone(ctx))
			return
		}

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

func shouldBypassTimeoutBuffering(r *http.Request) bool {
	if r.Method != http.MethodGet {
		return false
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != imageGenerationAssetContentPathSegmentCount {
		return false
	}

	resourceType := parts[imageGenerationAssetContentIndexAssets]
	return parts[imageGenerationAssetContentIndexAPI] == "api" &&
		parts[imageGenerationAssetContentIndexVersion] == "v1" &&
		parts[imageGenerationAssetContentIndexSessions] == "sessions" &&
		parts[imageGenerationAssetContentIndexSessionID] != "" &&
		(parts[imageGenerationAssetContentIndexGenerations] == "image-generations" ||
			parts[imageGenerationAssetContentIndexGenerations] == "presentation-generations") &&
		parts[imageGenerationAssetContentIndexGenerationID] != "" &&
		(resourceType == "assets" || (parts[imageGenerationAssetContentIndexGenerations] == "image-generations" && resourceType == "images")) &&
		parts[imageGenerationAssetContentIndexAssetID] != "" &&
		parts[imageGenerationAssetContentIndexContent] == "content"
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
