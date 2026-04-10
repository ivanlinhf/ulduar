package imagegen

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"strings"
	"time"

	"github.com/ivanlin/ulduar/apps/backend/internal/filenames"
	"github.com/ivanlin/ulduar/apps/backend/internal/repository"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	supportedInputMediaTypes = map[string]struct{}{
		"image/jpeg": {},
		"image/png":  {},
		"image/webp": {},
	}
)

type BlobStore interface {
	Upload(ctx context.Context, blobPath string, data []byte, contentType string) error
	Delete(ctx context.Context, blobPath string) error
}

type ValidationError struct {
	StatusCode int
	Message    string
}

func (e ValidationError) Error() string {
	return e.Message
}

type writeTx interface {
	GetSession(ctx context.Context, sessionID string) (repository.Session, error)
	CreateGeneration(ctx context.Context, params repository.CreateImageGenerationParams) (repository.ImageGeneration, error)
	CreateGenerationAsset(ctx context.Context, params repository.CreateImageGenerationAssetParams) (repository.ImageGenerationAsset, error)
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

type generationReader interface {
	GetByIDAndSession(ctx context.Context, generationID string, sessionID string) (repository.ImageGeneration, error)
}

type assetReader interface {
	ListByGenerationAndSession(ctx context.Context, generationID string, sessionID string) ([]repository.ImageGenerationAsset, error)
}

type Service struct {
	blobs                  BlobStore
	maxReferenceImageBytes int64
	beginWriteTxFn         func(ctx context.Context) (writeTx, error)
	generationRead         generationReader
	assetRead              assetReader
}

type ServiceOptions struct {
	MaxReferenceImageBytes int64
}

// NewService accepts zero or one ServiceOptions value.
// When options are omitted, defaults are applied.
func NewService(db *pgxpool.Pool, blobs BlobStore, options ...ServiceOptions) *Service {
	if len(options) > 1 {
		panic("imagegen.NewService accepts at most one ServiceOptions value")
	}

	resolvedOptions := ServiceOptions{}
	if len(options) > 0 {
		resolvedOptions = options[0]
	}

	service := &Service{
		blobs:                  blobs,
		maxReferenceImageBytes: resolvedOptions.MaxReferenceImageBytes,
	}
	if service.maxReferenceImageBytes <= 0 {
		service.maxReferenceImageBytes = DefaultMaxReferenceImageBytes
	}
	if db != nil {
		service.beginWriteTxFn = func(ctx context.Context) (writeTx, error) {
			tx, err := db.BeginTx(ctx, pgx.TxOptions{})
			if err != nil {
				return nil, fmt.Errorf("begin transaction: %w", err)
			}

			return repositoryWriteTx{tx: tx}, nil
		}
		service.generationRead = repository.NewImageGenerationRepository(db)
		service.assetRead = repository.NewImageGenerationAssetRepository(db)
	}

	return service
}

func (s *Service) CreatePendingGeneration(ctx context.Context, params CreateGenerationParams) (GenerationView, error) {
	if err := validateUUID(params.SessionID, "sessionId"); err != nil {
		return GenerationView{}, err
	}

	maxReferenceImageBytes := s.maxReferenceImageBytes
	if maxReferenceImageBytes <= 0 {
		maxReferenceImageBytes = DefaultMaxReferenceImageBytes
	}

	resolution, preparedAssets, prompt, err := validateCreateGenerationParams(params, maxReferenceImageBytes)
	if err != nil {
		return GenerationView{}, err
	}

	if s.beginWriteTxFn == nil {
		return GenerationView{}, fmt.Errorf("image generation service is not configured")
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

	generationRecord, err := tx.CreateGeneration(ctx, repository.CreateImageGenerationParams{
		SessionID:           params.SessionID,
		Mode:                string(params.Mode),
		Prompt:              prompt,
		ResolutionKey:       resolution.Key,
		Width:               resolution.Width,
		Height:              resolution.Height,
		RequestedImageCount: OutputImageCountV1,
		ProviderName:        "",
		ProviderModel:       "",
		Status:              string(StatusPending),
	})
	if err != nil {
		return GenerationView{}, fmt.Errorf("create image generation: %w", err)
	}

	if len(preparedAssets) > 0 && s.blobs == nil {
		return GenerationView{}, fmt.Errorf("blob store is not configured")
	}

	assetRecords := make([]repository.ImageGenerationAsset, 0, len(preparedAssets))
	for index, asset := range preparedAssets {
		blobPath := buildInputBlobPath(params.SessionID, generationRecord.ID, index, asset)
		if err := s.blobs.Upload(ctx, blobPath, asset.Data, asset.MediaType); err != nil {
			return GenerationView{}, fmt.Errorf("store reference image %q: %w", asset.Filename, err)
		}
		uploadedBlobPaths = append(uploadedBlobPaths, blobPath)

		assetRecord, err := tx.CreateGenerationAsset(ctx, repository.CreateImageGenerationAssetParams{
			GenerationID: generationRecord.ID,
			Role:         string(AssetRoleInput),
			SortOrder:    int64(index),
			BlobPath:     blobPath,
			MediaType:    asset.MediaType,
			Filename:     asset.Filename,
			SizeBytes:    asset.SizeBytes,
			Sha256:       asset.SHA256,
			Width:        asset.Width,
			Height:       asset.Height,
		})
		if err != nil {
			return GenerationView{}, fmt.Errorf("persist reference image %q: %w", asset.Filename, err)
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

func (s *Service) GetGeneration(ctx context.Context, sessionID, generationID string) (GenerationView, error) {
	if err := validateUUID(sessionID, "sessionId"); err != nil {
		return GenerationView{}, err
	}
	if err := validateUUID(generationID, "generationId"); err != nil {
		return GenerationView{}, err
	}
	if s.generationRead == nil || s.assetRead == nil {
		return GenerationView{}, fmt.Errorf("image generation service is not configured")
	}

	generationRecord, err := s.generationRead.GetByIDAndSession(ctx, generationID, sessionID)
	if err != nil {
		return GenerationView{}, mapRepositoryError(err, "image generation not found")
	}

	assetRecords, err := s.assetRead.ListByGenerationAndSession(ctx, generationID, sessionID)
	if err != nil {
		return GenerationView{}, fmt.Errorf("list image generation assets: %w", err)
	}

	return GenerationView{
		Generation: mapGeneration(generationRecord),
		Assets:     mapAssets(assetRecords),
	}, nil
}

type preparedAsset struct {
	Filename  string
	MediaType string
	SizeBytes int64
	SHA256    string
	Width     *int64
	Height    *int64
	Data      []byte
}

func validateCreateGenerationParams(params CreateGenerationParams, maxReferenceImageBytes int64) (Resolution, []preparedAsset, string, error) {
	prompt := strings.TrimSpace(params.Prompt)
	if prompt == "" {
		return Resolution{}, nil, "", ValidationError{
			StatusCode: http.StatusBadRequest,
			Message:    "prompt is required",
		}
	}

	resolution, ok := resolutionByKey(strings.TrimSpace(params.ResolutionKey))
	if !ok {
		return Resolution{}, nil, "", ValidationError{
			StatusCode: http.StatusBadRequest,
			Message:    fmt.Sprintf("resolution must be one of %s", supportedResolutionKeys()),
		}
	}

	switch params.Mode {
	case ModeTextToImage:
		if len(params.ReferenceImages) > 0 {
			return Resolution{}, nil, "", ValidationError{
				StatusCode: http.StatusBadRequest,
				Message:    "reference images are only supported for image_edit mode",
			}
		}
	case ModeImageEdit:
		if len(params.ReferenceImages) == 0 {
			return Resolution{}, nil, "", ValidationError{
				StatusCode: http.StatusBadRequest,
				Message:    "image_edit mode requires at least one reference image",
			}
		}
	default:
		return Resolution{}, nil, "", ValidationError{
			StatusCode: http.StatusBadRequest,
			Message:    "mode must be one of text_to_image or image_edit",
		}
	}

	if len(params.ReferenceImages) > MaxReferenceImages {
		return Resolution{}, nil, "", ValidationError{
			StatusCode: http.StatusBadRequest,
			Message:    fmt.Sprintf("too many reference images: maximum %d files", MaxReferenceImages),
		}
	}

	preparedAssets := make([]preparedAsset, 0, len(params.ReferenceImages))
	for _, upload := range params.ReferenceImages {
		asset, err := prepareInputAsset(upload, maxReferenceImageBytes)
		if err != nil {
			return Resolution{}, nil, "", err
		}

		preparedAssets = append(preparedAssets, asset)
	}

	return resolution, preparedAssets, prompt, nil
}

func prepareInputAsset(upload InputAssetUpload, maxReferenceImageBytes int64) (preparedAsset, error) {
	filename := filenames.Sanitize(upload.Filename, defaultAssetFilename)
	if len(upload.Data) == 0 {
		return preparedAsset{}, ValidationError{
			StatusCode: http.StatusBadRequest,
			Message:    fmt.Sprintf("reference image %q is empty", filename),
		}
	}

	if maxReferenceImageBytes > 0 && int64(len(upload.Data)) > maxReferenceImageBytes {
		return preparedAsset{}, ValidationError{
			StatusCode: http.StatusRequestEntityTooLarge,
			Message:    fmt.Sprintf("reference image %q exceeds %d bytes", filename, maxReferenceImageBytes),
		}
	}

	mediaType := http.DetectContentType(upload.Data)
	if _, ok := supportedInputMediaTypes[mediaType]; !ok {
		return preparedAsset{}, ValidationError{
			StatusCode: http.StatusUnsupportedMediaType,
			Message:    fmt.Sprintf("reference image %q has unsupported media type %q", filename, mediaType),
		}
	}

	sum := sha256.Sum256(upload.Data)
	width, height := detectImageDimensions(upload.Data)

	return preparedAsset{
		Filename:  filename,
		MediaType: mediaType,
		SizeBytes: int64(len(upload.Data)),
		SHA256:    hex.EncodeToString(sum[:]),
		Width:     width,
		Height:    height,
		Data:      upload.Data,
	}, nil
}

func buildInputBlobPath(sessionID, generationID string, index int, asset preparedAsset) string {
	return fmt.Sprintf(
		"sessions/%s/image-generations/%s/inputs/%02d-%s-%s",
		sessionID,
		generationID,
		index+1,
		asset.SHA256[:16],
		asset.Filename,
	)
}

func buildOutputBlobPath(sessionID, generationID, filename string) string {
	return fmt.Sprintf(
		"sessions/%s/image-generations/%s/outputs/%s",
		sessionID,
		generationID,
		filenames.Sanitize(filename, defaultAssetFilename),
	)
}

func resolutionByKey(key string) (Resolution, bool) {
	for _, resolution := range supportedResolutionCatalog {
		if resolution.Key == key {
			return resolution, true
		}
	}

	return Resolution{}, false
}

func supportedResolutionKeys() string {
	keys := make([]string, 0, len(supportedResolutionCatalog))
	for _, resolution := range supportedResolutionCatalog {
		keys = append(keys, resolution.Key)
	}

	return strings.Join(keys, ", ")
}

func detectImageDimensions(data []byte) (*int64, *int64) {
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, nil
	}

	width := int64(config.Width)
	height := int64(config.Height)

	return &width, &height
}

func mapGeneration(record repository.ImageGeneration) Generation {
	return Generation{
		ID:        record.ID,
		SessionID: record.SessionID,
		Mode:      Mode(record.Mode),
		Prompt:    record.Prompt,
		Resolution: Resolution{
			Key:    record.ResolutionKey,
			Width:  record.Width,
			Height: record.Height,
		},
		OutputImageCount: record.RequestedImageCount,
		ProviderName:     record.ProviderName,
		ProviderModel:    record.ProviderModel,
		ProviderJobID:    record.ProviderJobID,
		Status:           Status(record.Status),
		ErrorCode:        record.ErrorCode,
		ErrorMessage:     record.ErrorMessage,
		CreatedAt:        record.CreatedAt,
		CompletedAt:      record.CompletedAt,
	}
}

func mapAssets(records []repository.ImageGenerationAsset) []Asset {
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
			Width:        record.Width,
			Height:       record.Height,
			CreatedAt:    record.CreatedAt,
		})
	}

	return assets
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
	if s.blobs == nil || len(blobPaths) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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

func (t repositoryWriteTx) CreateGeneration(ctx context.Context, params repository.CreateImageGenerationParams) (repository.ImageGeneration, error) {
	return repository.NewImageGenerationRepository(t.tx).Create(ctx, params)
}

func (t repositoryWriteTx) CreateGenerationAsset(ctx context.Context, params repository.CreateImageGenerationAssetParams) (repository.ImageGenerationAsset, error) {
	return repository.NewImageGenerationAssetRepository(t.tx).Create(ctx, params)
}

func (t repositoryWriteTx) Commit(ctx context.Context) error {
	return t.tx.Commit(ctx)
}

func (t repositoryWriteTx) Rollback(ctx context.Context) error {
	return t.tx.Rollback(ctx)
}
