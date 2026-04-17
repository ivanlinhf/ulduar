package presentationgen

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/ivanlin/ulduar/apps/backend/internal/azureopenai"
	"github.com/ivanlin/ulduar/apps/backend/internal/blobstorage"
	"github.com/ivanlin/ulduar/apps/backend/internal/filenames"
	"github.com/ivanlin/ulduar/apps/backend/internal/presentationcompiler/pptx"
	"github.com/ivanlin/ulduar/apps/backend/internal/presentationdialect"
	"github.com/ivanlin/ulduar/apps/backend/internal/repository"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	plannerProviderName             = "azure-openai"
	providerMessageType             = "message"
	providerInputTextType           = "input_text"
	providerInputImageType          = "input_image"
	providerInputFileType           = "input_file"
	providerImageDetailLevel        = "auto"
	plannerUserRole                 = "user"
	plannerFailureInvalidJSON       = "invalid_planner_output"
	defaultMaxAttachments           = 5
	defaultMaxAttachmentBytes int64 = 20 << 20
	defaultOutputFilename           = "final.pptx"
	runningGracePeriod              = time.Minute
)

var (
	errPlannerInputAssetTooLarge             = errors.New("presentation input asset too large")
	errPlannerInputAssetDownload             = errors.New("presentation input asset download failed")
	errPlannerInputAssetUnsupportedMediaType = errors.New("presentation input asset unsupported media type")
	errPlannerProviderResponse               = errors.New("presentation planner provider response failed")
)

const plannerDialectInstructions = `You are the Ulduar v1 presentation planner.

Return exactly one JSON object and nothing else.
- Do not return Markdown fences, prose, comments, XML, or PPTX.
- The object must match the Ulduar v1 presentation dialect.
- Required top-level fields:
  - "version": "v1"
  - "slides": at least 1 slide
- Optional top-level field:
  - "slideSize": if present it must be "16:9"
- Supported slide layouts:
  - title
  - section
  - title_bullets
  - two_column
  - table
  - closing
- Every slide must have a non-empty "title".
- title and section slides may only use optional "subtitle".
- title_bullets slides must include "blocks", at least 1 block, and at least 1 block must be "bullet_list" or "numbered_list".
- two_column slides must include exactly 2 "columns"; each column may have optional "heading" and must have at least 1 block.
- table slides must include exactly 1 block and it must be a "table" block.
- closing slides may include optional "subtitle" and optional "blocks".
- Supported block types:
  - paragraph with non-empty "text"
  - bullet_list with non-empty "items"
  - numbered_list with non-empty "items"
  - table with non-empty "header" and non-empty "rows" where each row length matches the header length
  - quote with non-empty "text" and optional "attribution"
- Only paragraph, bullet_list, numbered_list, and quote blocks may appear in title_bullets, two_column, and closing slide text areas.
- Use attachments only as reference context. Do not embed uploaded assets in the JSON output.`

type BlobStore interface {
	Upload(ctx context.Context, blobPath string, data []byte, contentType string) error
	Delete(ctx context.Context, blobPath string) error
	Download(ctx context.Context, blobPath string) ([]byte, error)
	DownloadWithinLimit(ctx context.Context, blobPath string, maxBytes int64) ([]byte, error)
}

type ResponseClient interface {
	CreateResponse(ctx context.Context, request azureopenai.CreateResponseRequest) (azureopenai.Response, error)
}

type generationReader interface {
	GetByID(ctx context.Context, generationID string) (repository.PresentationGeneration, error)
	GetByIDAndSession(ctx context.Context, generationID string, sessionID string) (repository.PresentationGeneration, error)
	ClaimPending(ctx context.Context, params repository.ClaimPendingPresentationGenerationParams) (bool, error)
	UpdateState(ctx context.Context, params repository.UpdatePresentationGenerationStateParams) error
}

type assetReader interface {
	GetByIDAndSession(ctx context.Context, assetID string, sessionID string) (repository.PresentationGenerationAsset, error)
	ListByGeneration(ctx context.Context, generationID string) ([]repository.PresentationGenerationAsset, error)
	ListByGenerationAndSession(ctx context.Context, generationID string, sessionID string) ([]repository.PresentationGenerationAsset, error)
}

type writeTx interface {
	GetSession(ctx context.Context, sessionID string) (repository.Session, error)
	CreateGeneration(ctx context.Context, params repository.CreatePresentationGenerationParams) (repository.PresentationGeneration, error)
	CreateGenerationAsset(ctx context.Context, params repository.CreatePresentationGenerationAssetParams) (repository.PresentationGenerationAsset, error)
	LockGenerationForUpdate(ctx context.Context, generationID string) (repository.PresentationGeneration, error)
	UpdateGenerationState(ctx context.Context, params repository.UpdatePresentationGenerationStateParams) error
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

type Service struct {
	beginWriteTxFn func(ctx context.Context) (writeTx, error)
	planner        PlannerConfig
	blobs          BlobStore
	responses      ResponseClient
	generationRead generationReader
	assetRead      assetReader
}

type ServiceOptions struct {
	Planner        PlannerConfig
	BlobStore      BlobStore
	ResponseClient ResponseClient
}

type ValidationError struct {
	StatusCode int
	Message    string
}

func (e ValidationError) Error() string {
	return e.Message
}

// NewService accepts zero or one ServiceOptions value.
// When options are omitted, defaults are applied.
func NewService(db *pgxpool.Pool, options ...ServiceOptions) *Service {
	if len(options) > 1 {
		panic("presentationgen.NewService accepts at most one ServiceOptions value")
	}

	resolvedOptions := ServiceOptions{}
	if len(options) > 0 {
		resolvedOptions = options[0]
	}

	service := &Service{
		planner:   resolvedOptions.Planner,
		blobs:     resolvedOptions.BlobStore,
		responses: resolvedOptions.ResponseClient,
	}
	if db != nil {
		service.beginWriteTxFn = func(ctx context.Context) (writeTx, error) {
			tx, err := db.BeginTx(ctx, pgx.TxOptions{})
			if err != nil {
				return nil, fmt.Errorf("begin transaction: %w", err)
			}

			return repositoryWriteTx{tx: tx}, nil
		}
		service.generationRead = repository.NewPresentationGenerationRepository(db)
		service.assetRead = repository.NewPresentationGenerationAssetRepository(db)
	}

	return service
}

func (s *Service) Capabilities() Capabilities {
	return Capabilities{
		InputMediaTypes: SupportedInputMediaTypes(),
		OutputMediaType: OutputMediaTypePPTX,
		ProviderName:    plannerProviderName,
	}
}

func (s *Service) ProviderConfigured() bool {
	return s.PlannerConfigured()
}

func (s *Service) PlannerConfigured() bool {
	return strings.TrimSpace(s.planner.Endpoint) != "" && s.responses != nil
}

func (s *Service) CreatePendingGeneration(ctx context.Context, params CreateGenerationParams) (GenerationView, error) {
	if err := validateUUID(params.SessionID, "sessionId"); err != nil {
		return GenerationView{}, err
	}

	prompt, preparedAssets, err := validateCreateGenerationParams(params)
	if err != nil {
		return GenerationView{}, err
	}
	if s.beginWriteTxFn == nil {
		return GenerationView{}, fmt.Errorf("presentation generation service is not configured")
	}

	tx, err := s.beginWriteTxFn(ctx)
	if err != nil {
		return GenerationView{}, err
	}

	committed := false
	uploadedBlobPaths := make([]string, 0, len(preparedAssets))
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
			s.cleanupBlobs(uploadedBlobPaths)
		}
	}()

	if _, err := tx.GetSession(ctx, params.SessionID); err != nil {
		return GenerationView{}, mapRepositoryError(err, "session not found")
	}

	generationRecord, err := tx.CreateGeneration(ctx, repository.CreatePresentationGenerationParams{
		SessionID:     params.SessionID,
		Prompt:        prompt,
		ProviderName:  "",
		ProviderModel: "",
		Status:        string(StatusPending),
	})
	if err != nil {
		return GenerationView{}, fmt.Errorf("create presentation generation: %w", err)
	}

	if len(preparedAssets) > 0 && s.blobs == nil {
		return GenerationView{}, fmt.Errorf("blob store is not configured")
	}

	assetRecords := make([]repository.PresentationGenerationAsset, 0, len(preparedAssets))
	for index, asset := range preparedAssets {
		blobPath := buildInputBlobPath(params.SessionID, generationRecord.ID, index, asset)
		if err := s.blobs.Upload(ctx, blobPath, asset.Data, asset.MediaType); err != nil {
			return GenerationView{}, fmt.Errorf("store attachment %q: %w", asset.Filename, err)
		}
		uploadedBlobPaths = append(uploadedBlobPaths, blobPath)

		assetRecord, err := tx.CreateGenerationAsset(ctx, repository.CreatePresentationGenerationAssetParams{
			GenerationID: generationRecord.ID,
			Role:         string(AssetRoleInput),
			SortOrder:    int64(index),
			BlobPath:     blobPath,
			MediaType:    asset.MediaType,
			Filename:     asset.Filename,
			SizeBytes:    asset.SizeBytes,
			Sha256:       asset.SHA256,
		})
		if err != nil {
			return GenerationView{}, fmt.Errorf("persist attachment %q: %w", asset.Filename, err)
		}

		assetRecords = append(assetRecords, assetRecord)
	}

	if err := tx.Commit(ctx); err != nil {
		return GenerationView{}, fmt.Errorf("commit transaction: %w", err)
	}
	committed = true

	return GenerationView{
		Generation: mapGeneration(generationRecord),
		Assets:     mapAssets(assetRecords),
	}, nil
}

func (s *Service) ExecuteGeneration(ctx context.Context, generationID string) error {
	if s.responses == nil {
		return fmt.Errorf("presentation planner client is not configured")
	}
	if err := validateUUID(generationID, "generationId"); err != nil {
		return err
	}
	if s.generationRead == nil || s.assetRead == nil {
		return fmt.Errorf("presentation generation service is not configured")
	}

	generation, err := s.generationRead.GetByID(ctx, generationID)
	if err != nil {
		return mapRepositoryError(err, "presentation generation not found")
	}

	switch Status(generation.Status) {
	case StatusPending:
		return s.executePendingGeneration(ctx, generation)
	case StatusRunning:
		return s.executeRunningGeneration(ctx, generation)
	case StatusCompleted, StatusFailed:
		return nil
	default:
		return fmt.Errorf("unsupported presentation generation status %q", generation.Status)
	}
}

func (s *Service) executeRunningGeneration(ctx context.Context, generation repository.PresentationGeneration) error {
	if !runningGenerationStale(generation, s.planner.RequestTimeout) {
		return nil
	}

	return s.persistGenerationFailure(
		ctx,
		generation,
		"planner_stale_running",
		"presentation planner execution was interrupted and exceeded the allowed running window",
	)
}

func (s *Service) executePendingGeneration(ctx context.Context, generation repository.PresentationGeneration) error {
	providerModel := strings.TrimSpace(s.planner.Deployment)
	claimed, err := s.generationRead.ClaimPending(ctx, repository.ClaimPendingPresentationGenerationParams{
		ID:            generation.ID,
		ProviderName:  plannerProviderName,
		ProviderModel: providerModel,
	})
	if err != nil {
		return fmt.Errorf("claim pending presentation generation: %w", err)
	}
	if !claimed {
		return nil
	}
	generation.ProviderName = plannerProviderName
	generation.ProviderModel = providerModel
	generation.Status = string(StatusRunning)

	assets, err := s.assetRead.ListByGeneration(ctx, generation.ID)
	if err != nil {
		return s.failGenerationWithCause(ctx, generation, "list presentation generation assets", "input_asset_read_failed", err)
	}

	request, err := s.newCreateResponseRequest(ctx, generation, assets, "")
	if err != nil {
		return s.failGenerationWithCause(ctx, generation, "prepare presentation planner request", plannerRequestPreparationErrorCode(err), err)
	}

	response, dialectJSON, err := s.executePlannerRequest(ctx, request)
	if err != nil {
		return s.failGenerationWithCause(ctx, generation, "plan presentation", plannerErrorCode(err), err)
	}

	return s.completeGeneration(ctx, generation, response, dialectJSON)
}

func (s *Service) executePlannerRequest(ctx context.Context, request azureopenai.CreateResponseRequest) (azureopenai.Response, []byte, error) {
	response, plannerText, err := s.createPlannerResponse(ctx, request)
	if err != nil {
		return azureopenai.Response{}, nil, err
	}

	dialectJSON, validationErr := normalizePlannerOutput(plannerText)
	if validationErr == nil {
		return response, dialectJSON, nil
	}
	if !shouldRetryPlannerValidation(validationErr) {
		return response, nil, validationErr
	}

	originalInput, ok := request.Input.([]azureopenai.InputMessage)
	if !ok {
		return response, nil, fmt.Errorf("validate planner response JSON: unsupported planner input payload type %T", request.Input)
	}

	repairRequest := request
	repairRequest.Input = append(
		slices.Clone(originalInput),
		azureopenai.InputMessage{
			Type: providerMessageType,
			Role: plannerUserRole,
			Content: []azureopenai.InputContentItem{{
				Type: providerInputTextType,
				Text: fmt.Sprintf("Your previous response was invalid for the Ulduar presentation dialect: %s\n\nReturn a corrected JSON object only.", validationErr.Error()),
			}},
		},
	)

	repairResponse, repairedText, err := s.createPlannerResponse(ctx, repairRequest)
	if err != nil {
		return repairResponse, nil, err
	}

	dialectJSON, err = normalizePlannerOutput(repairedText)
	if err != nil {
		return repairResponse, nil, err
	}

	return repairResponse, dialectJSON, nil
}

func (s *Service) createPlannerResponse(ctx context.Context, request azureopenai.CreateResponseRequest) (azureopenai.Response, string, error) {
	requestCtx := ctx
	if s.planner.RequestTimeout > 0 {
		var cancel context.CancelFunc
		requestCtx, cancel = context.WithTimeout(ctx, s.planner.RequestTimeout)
		defer cancel()
	}

	response, err := s.responses.CreateResponse(requestCtx, request)
	if err != nil {
		return azureopenai.Response{}, "", err
	}
	if err := plannerResponseError(response); err != nil {
		return response, "", err
	}

	return response, extractResponseText(response), nil
}

func (s *Service) newCreateResponseRequest(ctx context.Context, generation repository.PresentationGeneration, assets []repository.PresentationGenerationAsset, repairMessage string) (azureopenai.CreateResponseRequest, error) {
	input, err := s.preparePlannerInput(ctx, generation, assets, repairMessage)
	if err != nil {
		return azureopenai.CreateResponseRequest{}, err
	}

	return azureopenai.CreateResponseRequest{
		Input:        input,
		Instructions: s.instructions(),
	}, nil
}

func (s *Service) preparePlannerInput(ctx context.Context, generation repository.PresentationGeneration, assets []repository.PresentationGenerationAsset, repairMessage string) ([]azureopenai.InputMessage, error) {
	content := make([]azureopenai.InputContentItem, 0, len(assets)+2)

	prompt := strings.TrimSpace(generation.Prompt)
	if prompt != "" {
		content = append(content, azureopenai.InputContentItem{
			Type: providerInputTextType,
			Text: fmt.Sprintf("Create a presentation plan for the following request. Use attached images and PDFs as reference material when relevant.\n\nUser request:\n%s", prompt),
		})
	}
	if repairMessage = strings.TrimSpace(repairMessage); repairMessage != "" {
		content = append(content, azureopenai.InputContentItem{
			Type: providerInputTextType,
			Text: fmt.Sprintf("Your previous response was invalid for the Ulduar presentation dialect: %s\n\nReturn a corrected JSON object only.", repairMessage),
		})
	}

	for _, asset := range sortInputAssets(assets) {
		item, err := s.prepareAttachmentInput(ctx, asset)
		if err != nil {
			return nil, err
		}
		content = append(content, item)
	}

	return []azureopenai.InputMessage{{
		Type:    providerMessageType,
		Role:    plannerUserRole,
		Content: content,
	}}, nil
}

func (s *Service) prepareAttachmentInput(ctx context.Context, asset repository.PresentationGenerationAsset) (azureopenai.InputContentItem, error) {
	if AssetRole(asset.Role) != AssetRoleInput {
		return azureopenai.InputContentItem{}, fmt.Errorf("unsupported presentation asset role %q", asset.Role)
	}
	if s.blobs == nil {
		return azureopenai.InputContentItem{}, fmt.Errorf("blob store is not configured")
	}
	if asset.SizeBytes > defaultMaxAttachmentBytes {
		return azureopenai.InputContentItem{}, fmt.Errorf("%w: input asset %s exceeds %d bytes", errPlannerInputAssetTooLarge, asset.ID, defaultMaxAttachmentBytes)
	}

	sanitizedFilename := filenames.Sanitize(asset.Filename, "attachment.pdf")
	switch asset.MediaType {
	case InputMediaTypeJPEG, InputMediaTypePNG, InputMediaTypeWEBP, InputMediaTypePDF:
	default:
		return azureopenai.InputContentItem{}, fmt.Errorf("%w: unsupported attachment media type %q", errPlannerInputAssetUnsupportedMediaType, asset.MediaType)
	}

	data, err := s.blobs.DownloadWithinLimit(ctx, asset.BlobPath, defaultMaxAttachmentBytes)
	if err != nil {
		if exceedsBlobDownloadLimit(err, defaultMaxAttachmentBytes) {
			return azureopenai.InputContentItem{}, fmt.Errorf("%w: input asset %s exceeds %d bytes", errPlannerInputAssetTooLarge, asset.ID, defaultMaxAttachmentBytes)
		}
		return azureopenai.InputContentItem{}, fmt.Errorf("%w: load input asset %s blob: %w", errPlannerInputAssetDownload, asset.ID, err)
	}

	switch asset.MediaType {
	case InputMediaTypeJPEG, InputMediaTypePNG, InputMediaTypeWEBP:
		return azureopenai.InputContentItem{
			Type:     providerInputImageType,
			ImageURL: buildDataURL(asset.MediaType, data),
			Detail:   providerImageDetailLevel,
		}, nil
	case InputMediaTypePDF:
		return azureopenai.InputContentItem{
			Type:     providerInputFileType,
			FileData: base64.StdEncoding.EncodeToString(data),
			Filename: sanitizedFilename,
		}, nil
	}

	return azureopenai.InputContentItem{}, fmt.Errorf("unsupported presentation attachment media type %q", asset.MediaType)
}

func (s *Service) instructions() string {
	parts := make([]string, 0, 2)
	if systemPrompt := strings.TrimSpace(s.planner.SystemPrompt); systemPrompt != "" {
		parts = append(parts, systemPrompt)
	}
	parts = append(parts, plannerDialectInstructions)
	return strings.Join(parts, "\n\n")
}

func normalizePlannerOutput(text string) ([]byte, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil, fmt.Errorf("planner response was empty")
	}

	document, err := presentationdialect.ParseJSON([]byte(trimmed))
	if err != nil {
		return nil, fmt.Errorf("validate planner response JSON: %w", err)
	}

	data, err := json.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("marshal normalized planner response: %w", err)
	}

	return data, nil
}

func shouldRetryPlannerValidation(err error) bool {
	var validationErr presentationdialect.ValidationError
	if errors.As(err, &validationErr) {
		return true
	}

	message := err.Error()
	return strings.Contains(message, "planner response was empty") ||
		strings.Contains(message, "decode presentation document:") ||
		strings.Contains(message, "unexpected trailing content") ||
		strings.Contains(message, "validate planner response JSON:")
}

func plannerErrorCode(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "provider_timeout"
	}

	var apiErr azureopenai.APIError
	if errors.As(err, &apiErr) {
		return fmt.Sprintf("provider_http_%d", apiErr.StatusCode)
	}

	if shouldRetryPlannerValidation(err) {
		return plannerFailureInvalidJSON
	}

	return "provider_request_failed"
}

func resolvedPlannerModel(response azureopenai.Response, fallback string) string {
	if model := strings.TrimSpace(response.Model); model != "" {
		return model
	}

	return strings.TrimSpace(fallback)
}

func sortInputAssets(assets []repository.PresentationGenerationAsset) []repository.PresentationGenerationAsset {
	inputAssets := make([]repository.PresentationGenerationAsset, 0, len(assets))
	for _, asset := range assets {
		if AssetRole(asset.Role) == AssetRoleInput {
			inputAssets = append(inputAssets, asset)
		}
	}

	sort.Slice(inputAssets, func(i, j int) bool {
		if inputAssets[i].SortOrder == inputAssets[j].SortOrder {
			return inputAssets[i].ID < inputAssets[j].ID
		}
		return inputAssets[i].SortOrder < inputAssets[j].SortOrder
	})

	return inputAssets
}

func buildDataURL(mediaType string, data []byte) string {
	return fmt.Sprintf("data:%s;base64,%s", mediaType, base64.StdEncoding.EncodeToString(data))
}

func plannerResponseError(response azureopenai.Response) error {
	if response.Error != nil {
		parts := make([]string, 0, 2)
		if code := strings.TrimSpace(response.Error.Code); code != "" {
			parts = append(parts, code)
		}
		if message := strings.TrimSpace(response.Error.Message); message != "" {
			parts = append(parts, message)
		}
		if len(parts) == 0 {
			return fmt.Errorf("%w: response returned an unspecified error", errPlannerProviderResponse)
		}
		return fmt.Errorf("%w: %s", errPlannerProviderResponse, strings.Join(parts, ": "))
	}

	status := strings.ToLower(strings.TrimSpace(response.Status))
	switch status {
	case "", "completed":
		return nil
	default:
		return fmt.Errorf("%w: unexpected response status %q", errPlannerProviderResponse, status)
	}
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

type preparedAsset struct {
	Filename  string
	MediaType string
	SizeBytes int64
	SHA256    string
	Data      []byte
}

func (s *Service) completeGeneration(ctx context.Context, generation repository.PresentationGeneration, response azureopenai.Response, dialectJSON []byte) error {
	if s.beginWriteTxFn == nil {
		return s.failGenerationWithCause(ctx, generation, "begin output persistence transaction", "persist_output_failed", fmt.Errorf("presentation generation service is not configured"))
	}
	if s.blobs == nil {
		return s.failGenerationWithCause(ctx, generation, "store output presentation", "store_output_failed", fmt.Errorf("blob store is not configured"))
	}

	outputAsset, err := prepareOutputAsset(dialectJSON)
	if err != nil {
		return s.failGenerationWithCause(ctx, generation, "compile output presentation", plannerFailureInvalidJSON, err)
	}

	tx, err := s.beginWriteTxFn(ctx)
	if err != nil {
		return s.failGenerationWithCause(ctx, generation, "begin output persistence transaction", "persist_output_failed", err)
	}
	lockedGeneration, err := tx.LockGenerationForUpdate(ctx, generation.ID)
	if err != nil {
		_ = tx.Rollback(ctx)
		return s.failGenerationWithCause(ctx, generation, "lock presentation generation for output persistence", "persist_output_failed", err)
	}
	if Status(lockedGeneration.Status) != StatusRunning || lockedGeneration.CompletedAt != nil {
		_ = tx.Rollback(ctx)
		return nil
	}
	generation = lockedGeneration

	providerModel := resolvedPlannerModel(response, generation.ProviderModel)
	blobPath := buildOutputBlobPath(generation.SessionID, generation.ID, outputAsset.Filename)
	if err := s.blobs.Upload(ctx, blobPath, outputAsset.Data, outputAsset.MediaType); err != nil {
		_ = tx.Rollback(ctx)
		return s.failGenerationWithCause(ctx, generation, "store output presentation", "store_output_failed", err)
	}
	rollbackAndCleanup := func() {
		_ = tx.Rollback(ctx)
		s.cleanupBlobs([]string{blobPath})
	}

	if _, err := tx.CreateGenerationAsset(ctx, repository.CreatePresentationGenerationAssetParams{
		GenerationID: generation.ID,
		Role:         string(AssetRoleOutput),
		SortOrder:    0,
		BlobPath:     blobPath,
		MediaType:    outputAsset.MediaType,
		Filename:     outputAsset.Filename,
		SizeBytes:    outputAsset.SizeBytes,
		Sha256:       outputAsset.SHA256,
	}); err != nil {
		rollbackAndCleanup()
		return s.failGenerationWithCause(ctx, generation, "persist output asset", "persist_output_failed", err)
	}

	completedAt := time.Now().UTC()
	if err := tx.UpdateGenerationState(ctx, repository.UpdatePresentationGenerationStateParams{
		ID:            generation.ID,
		ProviderName:  plannerProviderName,
		ProviderModel: providerModel,
		ProviderJobID: generation.ProviderJobID,
		Status:        string(StatusCompleted),
		CompletedAt:   &completedAt,
		DialectJSON:   dialectJSON,
	}); err != nil {
		rollbackAndCleanup()
		return s.failGenerationWithCause(ctx, generation, "mark presentation generation completed", "persist_output_failed", err)
	}

	if err := tx.Commit(ctx); err != nil {
		rollbackAndCleanup()
		return s.failGenerationWithCause(ctx, generation, "commit output persistence transaction", "persist_output_failed", err)
	}

	return nil
}

func validateCreateGenerationParams(params CreateGenerationParams) (string, []preparedAsset, error) {
	prompt := strings.TrimSpace(params.Prompt)
	if prompt == "" {
		return "", nil, ValidationError{
			StatusCode: http.StatusBadRequest,
			Message:    "prompt is required",
		}
	}
	if len(params.Attachments) > defaultMaxAttachments {
		return "", nil, ValidationError{
			StatusCode: http.StatusBadRequest,
			Message:    fmt.Sprintf("too many attachments: maximum %d files", defaultMaxAttachments),
		}
	}

	preparedAssets := make([]preparedAsset, 0, len(params.Attachments))
	for _, upload := range params.Attachments {
		asset, err := prepareInputAsset(upload)
		if err != nil {
			return "", nil, err
		}
		preparedAssets = append(preparedAssets, asset)
	}

	return prompt, preparedAssets, nil
}

func prepareInputAsset(upload InputAssetUpload) (preparedAsset, error) {
	filename := filenames.Sanitize(upload.Filename, "attachment")
	if len(upload.Data) == 0 {
		return preparedAsset{}, ValidationError{
			StatusCode: http.StatusBadRequest,
			Message:    fmt.Sprintf("attachment %q is empty", filename),
		}
	}
	if int64(len(upload.Data)) > defaultMaxAttachmentBytes {
		return preparedAsset{}, ValidationError{
			StatusCode: http.StatusRequestEntityTooLarge,
			Message:    fmt.Sprintf("attachment %q exceeds %d bytes", filename, defaultMaxAttachmentBytes),
		}
	}

	mediaType := strings.ToLower(strings.TrimSpace(http.DetectContentType(upload.Data)))
	switch mediaType {
	case InputMediaTypeJPEG, InputMediaTypePNG, InputMediaTypeWEBP, InputMediaTypePDF:
	default:
		return preparedAsset{}, ValidationError{
			StatusCode: http.StatusUnsupportedMediaType,
			Message:    fmt.Sprintf("attachment %q has unsupported media type %q", filename, mediaType),
		}
	}

	sum := sha256.Sum256(upload.Data)
	return preparedAsset{
		Filename:  filename,
		MediaType: mediaType,
		SizeBytes: int64(len(upload.Data)),
		SHA256:    hex.EncodeToString(sum[:]),
		Data:      slices.Clone(upload.Data),
	}, nil
}

func prepareOutputAsset(dialectJSON []byte) (preparedAsset, error) {
	document, err := presentationdialect.ParseJSON(dialectJSON)
	if err != nil {
		return preparedAsset{}, fmt.Errorf("decode normalized presentation document: %w", err)
	}
	data, err := pptx.Compile(document)
	if err != nil {
		return preparedAsset{}, fmt.Errorf("compile pptx: %w", err)
	}

	sum := sha256.Sum256(data)
	return preparedAsset{
		Filename:  defaultOutputFilename,
		MediaType: OutputMediaTypePPTX,
		SizeBytes: int64(len(data)),
		SHA256:    hex.EncodeToString(sum[:]),
		Data:      data,
	}, nil
}

func buildInputBlobPath(sessionID, generationID string, index int, asset preparedAsset) string {
	return fmt.Sprintf(
		"sessions/%s/presentation-generations/%s/inputs/%02d-%s-%s",
		sessionID,
		generationID,
		index+1,
		asset.SHA256[:16],
		asset.Filename,
	)
}

func buildOutputBlobPath(sessionID, generationID, filename string) string {
	return fmt.Sprintf(
		"sessions/%s/presentation-generations/%s/outputs/%s",
		sessionID,
		generationID,
		filenames.Sanitize(filename, defaultOutputFilename),
	)
}

func (s *Service) GetGeneration(ctx context.Context, sessionID, generationID string) (GenerationView, error) {
	if err := validateUUID(sessionID, "sessionId"); err != nil {
		return GenerationView{}, err
	}
	if err := validateUUID(generationID, "generationId"); err != nil {
		return GenerationView{}, err
	}
	if s.generationRead == nil || s.assetRead == nil {
		return GenerationView{}, fmt.Errorf("presentation generation service is not configured")
	}

	generationRecord, err := s.generationRead.GetByIDAndSession(ctx, generationID, sessionID)
	if err != nil {
		return GenerationView{}, mapRepositoryError(err, "presentation generation not found")
	}

	assetRecords, err := s.assetRead.ListByGenerationAndSession(ctx, generationID, sessionID)
	if err != nil {
		return GenerationView{}, fmt.Errorf("list presentation generation assets: %w", err)
	}

	return GenerationView{
		Generation: mapGeneration(generationRecord),
		Assets:     mapAssets(assetRecords),
	}, nil
}

func (s *Service) GetAssetContent(ctx context.Context, sessionID, generationID, assetID string) (AssetContent, error) {
	if err := validateUUID(sessionID, "sessionId"); err != nil {
		return AssetContent{}, err
	}
	if err := validateUUID(generationID, "generationId"); err != nil {
		return AssetContent{}, err
	}
	if err := validateUUID(assetID, "assetId"); err != nil {
		return AssetContent{}, err
	}
	if s.assetRead == nil {
		return AssetContent{}, fmt.Errorf("presentation generation service is not configured")
	}
	if s.blobs == nil {
		return AssetContent{}, fmt.Errorf("blob store is not configured")
	}

	assetRecord, err := s.assetRead.GetByIDAndSession(ctx, assetID, sessionID)
	if err != nil {
		return AssetContent{}, mapRepositoryError(err, "presentation generation asset not found")
	}
	if assetRecord.GenerationID != generationID || AssetRole(assetRecord.Role) != AssetRoleOutput {
		return AssetContent{}, ValidationError{
			StatusCode: http.StatusNotFound,
			Message:    "presentation generation asset not found",
		}
	}

	maxBytes := assetRecord.SizeBytes
	if maxBytes <= 0 {
		maxBytes = defaultMaxAttachmentBytes
	}
	data, err := s.blobs.DownloadWithinLimit(ctx, assetRecord.BlobPath, maxBytes)
	if err != nil {
		return AssetContent{}, fmt.Errorf("download presentation generation asset: %w", err)
	}

	return AssetContent{
		Filename:  assetRecord.Filename,
		MediaType: assetRecord.MediaType,
		Data:      data,
	}, nil
}

func mapGeneration(record repository.PresentationGeneration) Generation {
	return Generation{
		ID:            record.ID,
		SessionID:     record.SessionID,
		Prompt:        record.Prompt,
		DialectJSON:   slices.Clone(record.DialectJSON),
		ProviderName:  record.ProviderName,
		ProviderModel: record.ProviderModel,
		ProviderJobID: record.ProviderJobID,
		Status:        Status(record.Status),
		ErrorCode:     record.ErrorCode,
		ErrorMessage:  record.ErrorMessage,
		CreatedAt:     record.CreatedAt,
		StartedAt:     record.StartedAt,
		CompletedAt:   record.CompletedAt,
	}
}

func mapAssets(records []repository.PresentationGenerationAsset) []Asset {
	assets := make([]Asset, 0, len(records))
	for _, record := range records {
		assets = append(assets, Asset{
			ID:           record.ID,
			GenerationID: record.GenerationID,
			Role:         AssetRole(record.Role),
			SortOrder:    record.SortOrder,
			BlobPath:     record.BlobPath,
			MediaType:    record.MediaType,
			Filename:     record.Filename,
			SizeBytes:    record.SizeBytes,
			SHA256:       record.Sha256,
			CreatedAt:    record.CreatedAt,
		})
	}

	return assets
}

func (s *Service) failGenerationWithCause(ctx context.Context, generation repository.PresentationGeneration, action string, code string, cause error) error {
	if failErr := s.persistGenerationFailure(ctx, generation, code, cause.Error()); failErr != nil {
		return fmt.Errorf("%s: %w (also failed to persist presentation generation failure: %v)", action, cause, failErr)
	}

	return fmt.Errorf("%s: %w", action, cause)
}

func (s *Service) persistGenerationFailure(ctx context.Context, generation repository.PresentationGeneration, code string, message string) error {
	completedAt := time.Now().UTC()
	if err := s.generationRead.UpdateState(ctx, repository.UpdatePresentationGenerationStateParams{
		ID:            generation.ID,
		ProviderName:  strings.TrimSpace(generation.ProviderName),
		ProviderModel: strings.TrimSpace(generation.ProviderModel),
		ProviderJobID: generation.ProviderJobID,
		Status:        string(StatusFailed),
		ErrorCode:     strings.TrimSpace(code),
		ErrorMessage:  strings.TrimSpace(message),
		CompletedAt:   &completedAt,
		DialectJSON:   slices.Clone(generation.DialectJSON),
	}); err != nil {
		return fmt.Errorf("mark presentation generation failed: %w", err)
	}

	return nil
}

func (s *Service) cleanupBlobs(blobPaths []string) {
	if len(blobPaths) == 0 {
		return
	}
	if s.blobs == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, blobPath := range blobPaths {
		_ = s.blobs.Delete(ctx, blobPath)
	}
}

type repositoryWriteTx struct {
	tx pgx.Tx
}

func (t repositoryWriteTx) GetSession(ctx context.Context, sessionID string) (repository.Session, error) {
	return repository.NewSessionRepository(t.tx).GetByID(ctx, sessionID)
}

func (t repositoryWriteTx) CreateGeneration(ctx context.Context, params repository.CreatePresentationGenerationParams) (repository.PresentationGeneration, error) {
	return repository.NewPresentationGenerationRepository(t.tx).Create(ctx, params)
}

func (t repositoryWriteTx) CreateGenerationAsset(ctx context.Context, params repository.CreatePresentationGenerationAssetParams) (repository.PresentationGenerationAsset, error) {
	return repository.NewPresentationGenerationAssetRepository(t.tx).Create(ctx, params)
}

func (t repositoryWriteTx) LockGenerationForUpdate(ctx context.Context, generationID string) (repository.PresentationGeneration, error) {
	return repository.NewPresentationGenerationRepository(t.tx).LockForUpdate(ctx, generationID)
}

func (t repositoryWriteTx) UpdateGenerationState(ctx context.Context, params repository.UpdatePresentationGenerationStateParams) error {
	return repository.NewPresentationGenerationRepository(t.tx).UpdateState(ctx, params)
}

func (t repositoryWriteTx) Commit(ctx context.Context) error {
	return t.tx.Commit(ctx)
}

func (t repositoryWriteTx) Rollback(ctx context.Context) error {
	return t.tx.Rollback(ctx)
}

func runningGenerationStale(generation repository.PresentationGeneration, requestTimeout time.Duration) bool {
	if generation.StartedAt == nil {
		return true
	}

	if requestTimeout <= 0 {
		requestTimeout = 90 * time.Second
	}

	return time.Since(*generation.StartedAt) > requestTimeout+runningGracePeriod
}

func plannerRequestPreparationErrorCode(err error) string {
	switch {
	case errors.Is(err, errPlannerInputAssetTooLarge):
		return "input_asset_too_large"
	case errors.Is(err, errPlannerInputAssetDownload):
		return "input_asset_download_failed"
	case errors.Is(err, errPlannerInputAssetUnsupportedMediaType):
		return "input_asset_unsupported_media_type"
	default:
		return "planner_request_prepare_failed"
	}
}

func exceedsBlobDownloadLimit(err error, maxBytes int64) bool {
	return blobstorage.IsMaxBytesExceeded(err, maxBytes)
}

func validateUUID(value, field string) error {
	var parsed pgtype.UUID
	if err := parsed.Scan(strings.TrimSpace(value)); err != nil {
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
