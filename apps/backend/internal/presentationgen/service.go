package presentationgen

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ivanlin/ulduar/apps/backend/internal/repository"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type generationReader interface {
	GetByIDAndSession(ctx context.Context, generationID string, sessionID string) (repository.PresentationGeneration, error)
}

type assetReader interface {
	ListByGenerationAndSession(ctx context.Context, generationID string, sessionID string) ([]repository.PresentationGenerationAsset, error)
}

type Service struct {
	planner        PlannerConfig
	generationRead generationReader
	assetRead      assetReader
}

type ServiceOptions struct {
	Planner PlannerConfig
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
		planner: resolvedOptions.Planner,
	}
	if db != nil {
		service.generationRead = repository.NewPresentationGenerationRepository(db)
		service.assetRead = repository.NewPresentationGenerationAssetRepository(db)
	}

	return service
}

func (s *Service) PlannerConfigured() bool {
	return strings.TrimSpace(s.planner.Endpoint) != ""
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

func mapGeneration(record repository.PresentationGeneration) Generation {
	return Generation{
		ID:            record.ID,
		SessionID:     record.SessionID,
		Prompt:        record.Prompt,
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

func validateUUID(value, field string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ValidationError{
			StatusCode: 400,
			Message:    fmt.Sprintf("%s is required", field),
		}
	}
	var parsed pgtype.UUID
	if err := parsed.Scan(trimmed); err != nil {
		return ValidationError{
			StatusCode: 400,
			Message:    fmt.Sprintf("%s must be a valid UUID", field),
		}
	}

	return nil
}

func mapRepositoryError(err error, notFoundMessage string) error {
	if errors.Is(err, repository.ErrNotFound) {
		return ValidationError{
			StatusCode: 404,
			Message:    notFoundMessage,
		}
	}

	return err
}
