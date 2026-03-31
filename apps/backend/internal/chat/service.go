package chat

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ivanlin/ulduar/apps/backend/internal/azureopenai"
	"github.com/ivanlin/ulduar/apps/backend/internal/repository"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	messageRoleUser       = "user"
	messageRoleAssistant  = "assistant"
	messageStatusPending  = "pending"
	messageStatusComplete = "completed"
	messageStatusFailed   = "failed"
	runStatusPending      = "pending"
	runStatusStreaming    = "streaming"
	runStatusCompleted    = "completed"
	runStatusFailed       = "failed"
)

type BlobStore interface {
	Upload(ctx context.Context, blobPath string, data []byte, contentType string) error
	Delete(ctx context.Context, blobPath string) error
	Download(ctx context.Context, blobPath string) ([]byte, error)
}

type ResponseClient interface {
	CreateResponse(ctx context.Context, request azureopenai.CreateResponseRequest) (azureopenai.Response, error)
	StreamResponse(ctx context.Context, request azureopenai.CreateResponseRequest, onEvent func(azureopenai.StreamEvent) error) error
}

type ValidationError struct {
	StatusCode int
	Message    string
}

func (e ValidationError) Error() string {
	return e.Message
}

type Service struct {
	db                  *pgxpool.Pool
	blobs               BlobStore
	responses           ResponseClient
	instructions        string
	responseTimeout     time.Duration
	streamTimeout       time.Duration
	finalizationTimeout time.Duration
}

type ServiceOptions struct {
	Instructions        string
	ResponseTimeout     time.Duration
	StreamTimeout       time.Duration
	FinalizationTimeout time.Duration
}

type SessionView struct {
	Session  repository.Session
	Messages []MessageView
}

type MessageView struct {
	Message     repository.Message
	Content     MessageContent
	Attachments []repository.Attachment
}

type CreateMessageParams struct {
	SessionID   string
	Text        string
	Attachments []AttachmentUpload
}

type MessageCreation struct {
	UserMessage      repository.Message
	AssistantMessage repository.Message
	Run              repository.Run
	Attachments      []repository.Attachment
}

type RunStreamEvent struct {
	Type       string
	RunID      string
	MessageID  string
	ResponseID string
	ModelName  string
	Delta      string
	Error      string
	ErrorCode  string
}

func NewService(db *pgxpool.Pool, blobs BlobStore, responses ResponseClient, options ServiceOptions) *Service {
	return &Service{
		db:                  db,
		blobs:               blobs,
		responses:           responses,
		instructions:        strings.TrimSpace(options.Instructions),
		responseTimeout:     options.ResponseTimeout,
		streamTimeout:       options.StreamTimeout,
		finalizationTimeout: options.FinalizationTimeout,
	}
}

func (s *Service) CreateSession(ctx context.Context) (repository.Session, error) {
	repo := repository.NewSessionRepository(s.db)
	session, err := repo.Create(ctx)
	if err != nil {
		return repository.Session{}, fmt.Errorf("create session: %w", err)
	}

	return session, nil
}

func (s *Service) GetSession(ctx context.Context, sessionID string) (SessionView, error) {
	return s.loadSessionView(ctx, sessionID)
}

func (s *Service) loadSessionView(ctx context.Context, sessionID string) (SessionView, error) {
	if err := validateUUID(sessionID, "sessionId"); err != nil {
		return SessionView{}, err
	}

	sessionRepo := repository.NewSessionRepository(s.db)
	messageRepo := repository.NewMessageRepository(s.db)
	attachmentRepo := repository.NewAttachmentRepository(s.db)

	session, err := sessionRepo.GetByID(ctx, sessionID)
	if err != nil {
		return SessionView{}, mapRepositoryError(err, "session not found")
	}

	messages, err := messageRepo.ListBySession(ctx, sessionID)
	if err != nil {
		return SessionView{}, fmt.Errorf("list session messages: %w", err)
	}

	items := make([]MessageView, 0, len(messages))
	for _, message := range messages {
		content, err := DecodeContent(message.Content)
		if err != nil {
			return SessionView{}, fmt.Errorf("decode message %s content: %w", message.ID, err)
		}

		attachments, err := attachmentRepo.ListByMessage(ctx, message.ID)
		if err != nil {
			return SessionView{}, fmt.Errorf("list message %s attachments: %w", message.ID, err)
		}

		items = append(items, MessageView{
			Message:     message,
			Content:     content,
			Attachments: attachments,
		})
	}

	return SessionView{
		Session:  session,
		Messages: items,
	}, nil
}

func (s *Service) CreateMessage(ctx context.Context, params CreateMessageParams) (MessageCreation, error) {
	if err := validateUUID(params.SessionID, "sessionId"); err != nil {
		return MessageCreation{}, err
	}
	if err := validateCreateMessageParams(params); err != nil {
		return MessageCreation{}, err
	}

	userContent, err := NewTextContent(params.Text)
	if err != nil {
		return MessageCreation{}, err
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return MessageCreation{}, fmt.Errorf("begin transaction: %w", err)
	}

	committed := false
	uploadedBlobPaths := make([]string, 0, len(params.Attachments))
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
			s.cleanupBlobs(uploadedBlobPaths)
		}
	}()

	sessionRepo := repository.NewSessionRepository(tx)
	messageRepo := repository.NewMessageRepository(tx)
	attachmentRepo := repository.NewAttachmentRepository(tx)
	runRepo := repository.NewRunRepository(tx)

	if _, err := sessionRepo.GetByID(ctx, params.SessionID); err != nil {
		return MessageCreation{}, mapRepositoryError(err, "session not found")
	}

	userMessage, err := messageRepo.Create(ctx, repository.CreateMessageParams{
		SessionID: params.SessionID,
		Role:      messageRoleUser,
		Content:   userContent,
		Status:    messageStatusComplete,
	})
	if err != nil {
		return MessageCreation{}, fmt.Errorf("create user message: %w", err)
	}

	attachments := make([]repository.Attachment, 0, len(params.Attachments))
	for index, attachment := range params.Attachments {
		blobPath := buildBlobPath(params.SessionID, userMessage.ID, index, attachment)
		if err := s.blobs.Upload(ctx, blobPath, attachment.Data, attachment.MediaType); err != nil {
			return MessageCreation{}, fmt.Errorf("store attachment %q: %w", attachment.Filename, err)
		}
		uploadedBlobPaths = append(uploadedBlobPaths, blobPath)

		record, err := attachmentRepo.Create(ctx, repository.CreateAttachmentParams{
			SessionID: params.SessionID,
			MessageID: userMessage.ID,
			BlobPath:  blobPath,
			MediaType: attachment.MediaType,
			Filename:  attachment.Filename,
			SizeBytes: attachment.SizeBytes,
			Sha256:    attachment.SHA256,
		})
		if err != nil {
			return MessageCreation{}, fmt.Errorf("persist attachment %q: %w", attachment.Filename, err)
		}

		attachments = append(attachments, record)
	}

	assistantMessage, err := messageRepo.Create(ctx, repository.CreateMessageParams{
		SessionID: params.SessionID,
		Role:      messageRoleAssistant,
		Content:   NewEmptyContent(),
		Status:    messageStatusPending,
	})
	if err != nil {
		return MessageCreation{}, fmt.Errorf("create assistant placeholder: %w", err)
	}

	startedAt := time.Now().UTC()
	run, err := runRepo.Create(ctx, repository.CreateRunParams{
		SessionID:          params.SessionID,
		UserMessageID:      userMessage.ID,
		AssistantMessageID: assistantMessage.ID,
		Status:             runStatusPending,
		StartedAt:          startedAt,
	})
	if err != nil {
		return MessageCreation{}, fmt.Errorf("create run: %w", err)
	}

	if err := sessionRepo.TouchLastMessageAt(ctx, params.SessionID); err != nil {
		return MessageCreation{}, mapRepositoryError(err, "session not found")
	}

	if err := tx.Commit(ctx); err != nil {
		return MessageCreation{}, fmt.Errorf("commit transaction: %w", err)
	}
	committed = true

	return MessageCreation{
		UserMessage:      userMessage,
		AssistantMessage: assistantMessage,
		Run:              run,
		Attachments:      attachments,
	}, nil
}

func (s *Service) ExecuteRun(ctx context.Context, runID string) error {
	if s.responses == nil {
		return fmt.Errorf("responses client is not configured")
	}
	if err := validateUUID(runID, "runId"); err != nil {
		return err
	}

	runRepo := repository.NewRunRepository(s.db)
	messageRepo := repository.NewMessageRepository(s.db)

	run, err := runRepo.GetByID(ctx, runID)
	if err != nil {
		return mapRepositoryError(err, "run not found")
	}
	if run.AssistantMessageID == "" {
		return fmt.Errorf("run %s has no assistant message placeholder", run.ID)
	}
	if run.Status != runStatusPending {
		return nil
	}

	input, err := s.ReconstructConversationForTurn(ctx, run.SessionID, run.UserMessageID)
	if err != nil {
		if failErr := s.failRun(ctx, run, "conversation_reconstruction_failed"); failErr != nil {
			return fmt.Errorf("reconstruct conversation: %w (also failed to persist run failure: %v)", err, failErr)
		}
		return fmt.Errorf("reconstruct conversation: %w", err)
	}

	responseCtx, cancel := s.withOperationTimeout(ctx, s.responseTimeout)
	defer cancel()

	response, err := s.responses.CreateResponse(responseCtx, azureopenai.CreateResponseRequest{
		Input:        input,
		Instructions: s.instructions,
	})
	if err != nil {
		code := providerErrorCode(err)
		if failErr := s.failRun(ctx, run, code); failErr != nil {
			return fmt.Errorf("create response: %w (also failed to persist run failure: %v)", err, failErr)
		}
		return fmt.Errorf("create response: %w", err)
	}

	outputText := extractResponseText(response)
	content, err := NewTextContent(outputText)
	if err != nil {
		if failErr := s.failRun(ctx, run, "invalid_provider_output"); failErr != nil {
			return fmt.Errorf("encode assistant content: %w (also failed to persist run failure: %v)", err, failErr)
		}
		return fmt.Errorf("encode assistant content: %w", err)
	}

	completedAt := time.Now().UTC()
	if err := messageRepo.UpdateContentAndState(ctx, repository.UpdateMessageContentStateParams{
		ID:        run.AssistantMessageID,
		Content:   content,
		Status:    messageStatusComplete,
		ModelName: response.Model,
	}); err != nil {
		if failErr := s.failRun(ctx, run, "persist_assistant_message_failed"); failErr != nil {
			return fmt.Errorf("update assistant message: %w (also failed to persist run failure: %v)", err, failErr)
		}
		return fmt.Errorf("update assistant message: %w", err)
	}

	if err := runRepo.UpdateState(ctx, repository.UpdateRunStateParams{
		ID:                 run.ID,
		AssistantMessageID: run.AssistantMessageID,
		ProviderResponseID: response.ID,
		Status:             runStatusCompleted,
		CompletedAt:        &completedAt,
	}); err != nil {
		return fmt.Errorf("update completed run: %w", err)
	}

	return nil
}

func (s *Service) StreamRun(ctx context.Context, sessionID, runID string, emit func(RunStreamEvent) error) error {
	if s.responses == nil {
		return fmt.Errorf("responses client is not configured")
	}
	if err := validateUUID(sessionID, "sessionId"); err != nil {
		return err
	}
	if err := validateUUID(runID, "runId"); err != nil {
		return err
	}

	run, err := s.loadRunForSession(ctx, sessionID, runID)
	if err != nil {
		return err
	}

	switch run.Status {
	case runStatusCompleted:
		return s.replayCompletedRun(ctx, run, emit)
	case runStatusFailed:
		return emit(RunStreamEvent{
			Type:      "run.failed",
			RunID:     run.ID,
			MessageID: run.AssistantMessageID,
			Error:     "run failed",
			ErrorCode: run.ErrorCode,
		})
	case runStatusStreaming:
		return ValidationError{
			StatusCode: http.StatusConflict,
			Message:    "run is already streaming",
		}
	case runStatusPending:
		return s.executeStreamRun(ctx, run, emit)
	default:
		return fmt.Errorf("unsupported run status %q", run.Status)
	}
}

func buildBlobPath(sessionID, messageID string, index int, attachment AttachmentUpload) string {
	return fmt.Sprintf(
		"sessions/%s/messages/%s/attachments/%02d-%s-%s",
		sessionID,
		messageID,
		index+1,
		attachment.SHA256[:16],
		attachment.Filename,
	)
}

func validateCreateMessageParams(params CreateMessageParams) error {
	if len(params.Attachments) == 0 && strings.TrimSpace(params.Text) == "" {
		return ValidationError{
			StatusCode: http.StatusBadRequest,
			Message:    "message text or at least one attachment is required",
		}
	}

	if len(params.Attachments) > MaxAttachmentsPerMessage {
		return ValidationError{
			StatusCode: http.StatusBadRequest,
			Message:    fmt.Sprintf("too many attachments: maximum %d files", MaxAttachmentsPerMessage),
		}
	}

	return nil
}

func (s *Service) loadRunForSession(ctx context.Context, sessionID, runID string) (repository.Run, error) {
	runRepo := repository.NewRunRepository(s.db)
	run, err := runRepo.GetByID(ctx, runID)
	if err != nil {
		return repository.Run{}, mapRepositoryError(err, "run not found")
	}
	if run.SessionID != sessionID {
		return repository.Run{}, ValidationError{
			StatusCode: http.StatusNotFound,
			Message:    "run not found",
		}
	}

	return run, nil
}

func (s *Service) replayCompletedRun(ctx context.Context, run repository.Run, emit func(RunStreamEvent) error) error {
	messageRepo := repository.NewMessageRepository(s.db)
	assistantMessage, err := messageRepo.GetByID(ctx, run.AssistantMessageID)
	if err != nil {
		return mapRepositoryError(err, "run not found")
	}

	content, err := DecodeContent(assistantMessage.Content)
	if err != nil {
		return fmt.Errorf("decode assistant message content: %w", err)
	}

	text := contentText(content)
	if text != "" {
		if err := emit(RunStreamEvent{
			Type:      "message.delta",
			RunID:     run.ID,
			MessageID: run.AssistantMessageID,
			Delta:     text,
		}); err != nil {
			return err
		}
	}

	return emit(RunStreamEvent{
		Type:       "run.completed",
		RunID:      run.ID,
		MessageID:  run.AssistantMessageID,
		ResponseID: run.ProviderResponseID,
		ModelName:  assistantMessage.ModelName,
	})
}

func (s *Service) executeStreamRun(ctx context.Context, run repository.Run, emit func(RunStreamEvent) error) error {
	runRepo := repository.NewRunRepository(s.db)

	if err := runRepo.UpdateState(ctx, repository.UpdateRunStateParams{
		ID:                 run.ID,
		AssistantMessageID: run.AssistantMessageID,
		Status:             runStatusStreaming,
	}); err != nil {
		return fmt.Errorf("mark run streaming: %w", err)
	}

	input, err := s.ReconstructConversationForTurn(ctx, run.SessionID, run.UserMessageID)
	if err != nil {
		if failErr := s.failRun(ctx, run, "conversation_reconstruction_failed"); failErr != nil {
			return fmt.Errorf("reconstruct conversation: %w (also failed to persist run failure: %v)", err, failErr)
		}
		return fmt.Errorf("reconstruct conversation: %w", err)
	}

	var (
		responseMeta   azureopenai.Response
		seenCreated    bool
		seenCompleted  bool
		seenFailure    bool
		failureCode    string
		failureMessage string
		textBuilder    strings.Builder
	)

	streamCtx, cancel := s.withOperationTimeout(ctx, s.streamTimeout)
	defer cancel()

	err = s.responses.StreamResponse(streamCtx, azureopenai.CreateResponseRequest{
		Input:        input,
		Instructions: s.instructions,
	}, func(event azureopenai.StreamEvent) error {
		switch event.Type {
		case "response.created":
			if event.Response != nil {
				responseMeta = *event.Response
				seenCreated = true
				if err := runRepo.UpdateState(ctx, repository.UpdateRunStateParams{
					ID:                 run.ID,
					AssistantMessageID: run.AssistantMessageID,
					ProviderResponseID: event.Response.ID,
					Status:             runStatusStreaming,
				}); err != nil {
					return fmt.Errorf("update streaming run metadata: %w", err)
				}
			}

			return emit(RunStreamEvent{
				Type:       "run.started",
				RunID:      run.ID,
				MessageID:  run.AssistantMessageID,
				ResponseID: responseMeta.ID,
				ModelName:  responseMeta.Model,
			})
		case "response.output_text.delta":
			if event.Delta == "" {
				return nil
			}
			textBuilder.WriteString(event.Delta)
			return emit(RunStreamEvent{
				Type:      "message.delta",
				RunID:     run.ID,
				MessageID: run.AssistantMessageID,
				Delta:     event.Delta,
			})
		case "response.completed":
			if event.Response != nil {
				responseMeta = *event.Response
			}
			seenCompleted = true
			return nil
		case "response.failed", "response.incomplete", "error":
			seenFailure = true
			if event.Response != nil {
				responseMeta = *event.Response
				if event.Response.Error != nil {
					failureCode = event.Response.Error.Code
					failureMessage = event.Response.Error.Message
				}
			}
			if event.Error != nil {
				failureCode = event.Error.Code
				failureMessage = event.Error.Message
			}
			return nil
		default:
			return nil
		}
	})
	if err != nil {
		code := providerErrorCode(err)
		if failErr := s.failRun(ctx, run, code); failErr != nil {
			return fmt.Errorf("stream response: %w (also failed to persist run failure: %v)", err, failErr)
		}
		return emit(RunStreamEvent{
			Type:      "run.failed",
			RunID:     run.ID,
			MessageID: run.AssistantMessageID,
			Error:     err.Error(),
			ErrorCode: code,
		})
	}

	if seenFailure || !seenCompleted {
		if failureCode == "" {
			failureCode = "provider_stream_failed"
		}
		if failureMessage == "" {
			failureMessage = "provider stream failed"
		}
		if failErr := s.failRun(ctx, run, failureCode); failErr != nil {
			return fmt.Errorf("persist failed run: %w", failErr)
		}
		return emit(RunStreamEvent{
			Type:       "run.failed",
			RunID:      run.ID,
			MessageID:  run.AssistantMessageID,
			ResponseID: responseMeta.ID,
			Error:      failureMessage,
			ErrorCode:  failureCode,
		})
	}

	if !seenCreated && responseMeta.ID == "" {
		responseMeta.ID = run.ProviderResponseID
	}

	finalText := strings.TrimSpace(textBuilder.String())
	if finalText == "" {
		finalText = extractResponseText(responseMeta)
	}

	content, err := NewTextContent(finalText)
	if err != nil {
		if failErr := s.failRun(ctx, run, "invalid_provider_output"); failErr != nil {
			return fmt.Errorf("encode assistant content: %w (also failed to persist run failure: %v)", err, failErr)
		}
		return fmt.Errorf("encode assistant content: %w", err)
	}

	if err := s.completeRun(ctx, run, responseMeta, content); err != nil {
		if failErr := s.failRun(ctx, run, "persist_assistant_message_failed"); failErr != nil {
			return fmt.Errorf("complete run: %w (also failed to persist run failure: %v)", err, failErr)
		}
		return fmt.Errorf("complete run: %w", err)
	}

	return emit(RunStreamEvent{
		Type:       "run.completed",
		RunID:      run.ID,
		MessageID:  run.AssistantMessageID,
		ResponseID: responseMeta.ID,
		ModelName:  responseMeta.Model,
	})
}

func (s *Service) failRun(ctx context.Context, run repository.Run, errorCode string) error {
	ctx, cancel := s.withFinalizationTimeout(ctx)
	defer cancel()

	messageRepo := repository.NewMessageRepository(s.db)
	runRepo := repository.NewRunRepository(s.db)
	completedAt := time.Now().UTC()

	if err := messageRepo.UpdateState(ctx, repository.UpdateMessageStateParams{
		ID:     run.AssistantMessageID,
		Status: messageStatusFailed,
	}); err != nil {
		return fmt.Errorf("mark assistant message failed: %w", err)
	}

	if err := runRepo.UpdateState(ctx, repository.UpdateRunStateParams{
		ID:                 run.ID,
		AssistantMessageID: run.AssistantMessageID,
		Status:             runStatusFailed,
		ErrorCode:          errorCode,
		CompletedAt:        &completedAt,
	}); err != nil {
		return fmt.Errorf("mark run failed: %w", err)
	}

	return nil
}

func (s *Service) completeRun(ctx context.Context, run repository.Run, response azureopenai.Response, content []byte) error {
	ctx, cancel := s.withFinalizationTimeout(ctx)
	defer cancel()

	messageRepo := repository.NewMessageRepository(s.db)
	runRepo := repository.NewRunRepository(s.db)
	completedAt := time.Now().UTC()

	if err := messageRepo.UpdateContentAndState(ctx, repository.UpdateMessageContentStateParams{
		ID:        run.AssistantMessageID,
		Content:   content,
		Status:    messageStatusComplete,
		ModelName: response.Model,
	}); err != nil {
		return fmt.Errorf("update assistant message: %w", err)
	}

	if err := runRepo.UpdateState(ctx, repository.UpdateRunStateParams{
		ID:                 run.ID,
		AssistantMessageID: run.AssistantMessageID,
		ProviderResponseID: response.ID,
		Status:             runStatusCompleted,
		CompletedAt:        &completedAt,
	}); err != nil {
		return fmt.Errorf("update completed run: %w", err)
	}

	return nil
}

func providerErrorCode(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "provider_timeout"
	}

	var apiErr azureopenai.APIError
	if errors.As(err, &apiErr) {
		return fmt.Sprintf("provider_http_%d", apiErr.StatusCode)
	}

	return "provider_request_failed"
}

func extractResponseText(response azureopenai.Response) string {
	if trimmed := strings.TrimSpace(response.OutputText); trimmed != "" {
		return trimmed
	}

	var builder strings.Builder
	for _, item := range response.Output {
		for _, content := range item.Content {
			if content.Type != "output_text" && content.Type != "text" {
				continue
			}
			text := strings.TrimSpace(content.Text)
			if text == "" {
				continue
			}
			if builder.Len() > 0 {
				builder.WriteString("\n\n")
			}
			builder.WriteString(text)
		}
	}

	return builder.String()
}

func contentText(content MessageContent) string {
	var builder strings.Builder
	for _, part := range content.Parts {
		if part.Type != contentPartTypeText {
			continue
		}
		text := strings.TrimSpace(part.Text)
		if text == "" {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteString("\n\n")
		}
		builder.WriteString(text)
	}

	return builder.String()
}

func validateUUID(value, field string) error {
	var uuid pgtype.UUID
	if err := uuid.Scan(strings.TrimSpace(value)); err != nil {
		return ValidationError{
			StatusCode: http.StatusBadRequest,
			Message:    fmt.Sprintf("%s must be a valid UUID", field),
		}
	}

	return nil
}

func mapRepositoryError(err error, notFoundMessage string) error {
	if errors.Is(err, repository.ErrNotFound) {
		return ValidationError{
			StatusCode: http.StatusNotFound,
			Message:    notFoundMessage,
		}
	}

	return err
}

func (s *Service) cleanupBlobs(blobPaths []string) {
	if len(blobPaths) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, blobPath := range blobPaths {
		_ = s.blobs.Delete(ctx, blobPath)
	}
}

func (s *Service) withOperationTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return ctx, func() {}
	}

	return context.WithTimeout(ctx, timeout)
}

func (s *Service) withFinalizationTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	base := context.WithoutCancel(ctx)
	if s.finalizationTimeout <= 0 {
		return base, func() {}
	}

	return context.WithTimeout(base, s.finalizationTimeout)
}
