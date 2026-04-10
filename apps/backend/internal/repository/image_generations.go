package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ivanlin/ulduar/apps/backend/internal/dbsqlc"
	"github.com/jackc/pgx/v5"
)

type ImageGeneration struct {
	ID                  string
	SessionID           string
	Mode                string
	Prompt              string
	ResolutionKey       string
	Width               int64
	Height              int64
	RequestedImageCount int64
	ProviderName        string
	ProviderModel       string
	ProviderJobID       string
	Status              string
	ErrorCode           string
	ErrorMessage        string
	CreatedAt           time.Time
	CompletedAt         *time.Time
}

type CreateImageGenerationParams struct {
	SessionID           string
	Mode                string
	Prompt              string
	ResolutionKey       string
	Width               int64
	Height              int64
	RequestedImageCount int64
	ProviderName        string
	ProviderModel       string
	ProviderJobID       string
	Status              string
	ErrorCode           string
	ErrorMessage        string
	CompletedAt         *time.Time
}

type UpdateImageGenerationStateParams struct {
	ProviderName  string
	ProviderModel string
	ID            string
	ProviderJobID string
	Status        string
	ErrorCode     string
	ErrorMessage  string
	CompletedAt   *time.Time
}

type ClaimPendingImageGenerationParams struct {
	ID            string
	ProviderName  string
	ProviderModel string
}

type ImageGenerationRepository struct {
	queries *dbsqlc.Queries
}

func NewImageGenerationRepository(db dbsqlc.DBTX) *ImageGenerationRepository {
	return &ImageGenerationRepository{
		queries: dbsqlc.New(db),
	}
}

func (r *ImageGenerationRepository) Create(ctx context.Context, params CreateImageGenerationParams) (ImageGeneration, error) {
	sessionID, err := parseUUID(params.SessionID)
	if err != nil {
		return ImageGeneration{}, fmt.Errorf("parse session id: %w", err)
	}

	row, err := r.queries.CreateImageGeneration(ctx, dbsqlc.CreateImageGenerationParams{
		SessionID:           sessionID,
		Mode:                params.Mode,
		Prompt:              params.Prompt,
		ResolutionKey:       params.ResolutionKey,
		Width:               params.Width,
		Height:              params.Height,
		RequestedImageCount: params.RequestedImageCount,
		ProviderName:        params.ProviderName,
		ProviderModel:       params.ProviderModel,
		ProviderJobID:       textValue(params.ProviderJobID),
		Status:              params.Status,
		ErrorCode:           textValue(params.ErrorCode),
		ErrorMessage:        textValue(params.ErrorMessage),
		CompletedAt:         timestamptzPointerValue(params.CompletedAt),
	})
	if err != nil {
		return ImageGeneration{}, fmt.Errorf("create image generation: %w", err)
	}

	generation, err := mapImageGeneration(row)
	if err != nil {
		return ImageGeneration{}, fmt.Errorf("map created image generation: %w", err)
	}

	return generation, nil
}

func (r *ImageGenerationRepository) GetByID(ctx context.Context, generationID string) (ImageGeneration, error) {
	id, err := parseUUID(generationID)
	if err != nil {
		return ImageGeneration{}, fmt.Errorf("parse image generation id: %w", err)
	}

	row, err := r.queries.GetImageGeneration(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ImageGeneration{}, ErrNotFound
	}
	if err != nil {
		return ImageGeneration{}, fmt.Errorf("get image generation %s: %w", generationID, err)
	}

	generation, err := mapImageGeneration(row)
	if err != nil {
		return ImageGeneration{}, fmt.Errorf("map image generation %s: %w", generationID, err)
	}

	return generation, nil
}

func (r *ImageGenerationRepository) GetByIDAndSession(ctx context.Context, generationID string, sessionID string) (ImageGeneration, error) {
	generationUUID, err := parseUUID(generationID)
	if err != nil {
		return ImageGeneration{}, fmt.Errorf("parse image generation id: %w", err)
	}

	sessionUUID, err := parseUUID(sessionID)
	if err != nil {
		return ImageGeneration{}, fmt.Errorf("parse session id: %w", err)
	}

	row, err := r.queries.GetImageGenerationBySession(ctx, dbsqlc.GetImageGenerationBySessionParams{
		ID:        generationUUID,
		SessionID: sessionUUID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ImageGeneration{}, ErrNotFound
	}
	if err != nil {
		return ImageGeneration{}, fmt.Errorf("get image generation %s for session %s: %w", generationID, sessionID, err)
	}

	generation, err := mapImageGeneration(row)
	if err != nil {
		return ImageGeneration{}, fmt.Errorf("map image generation %s for session %s: %w", generationID, sessionID, err)
	}

	return generation, nil
}

func (r *ImageGenerationRepository) ListBySession(ctx context.Context, sessionID string) ([]ImageGeneration, error) {
	id, err := parseUUID(sessionID)
	if err != nil {
		return nil, fmt.Errorf("parse session id: %w", err)
	}

	rows, err := r.queries.ListImageGenerationsBySession(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list image generations for session %s: %w", sessionID, err)
	}

	generations := make([]ImageGeneration, 0, len(rows))
	for _, row := range rows {
		generation, err := mapImageGeneration(row)
		if err != nil {
			return nil, fmt.Errorf("map image generation for session %s: %w", sessionID, err)
		}
		generations = append(generations, generation)
	}

	return generations, nil
}

func (r *ImageGenerationRepository) ClaimPending(ctx context.Context, params ClaimPendingImageGenerationParams) (bool, error) {
	id, err := parseUUID(params.ID)
	if err != nil {
		return false, fmt.Errorf("parse image generation id: %w", err)
	}

	rowsAffected, err := r.queries.ClaimPendingImageGeneration(ctx, dbsqlc.ClaimPendingImageGenerationParams{
		ID:            id,
		ProviderName:  params.ProviderName,
		ProviderModel: params.ProviderModel,
	})
	if err != nil {
		return false, fmt.Errorf("claim pending image generation %s: %w", params.ID, err)
	}

	return rowsAffected > 0, nil
}

func (r *ImageGenerationRepository) UpdateState(ctx context.Context, params UpdateImageGenerationStateParams) error {
	id, err := parseUUID(params.ID)
	if err != nil {
		return fmt.Errorf("parse image generation id: %w", err)
	}

	rowsAffected, err := r.queries.UpdateImageGenerationState(ctx, dbsqlc.UpdateImageGenerationStateParams{
		ProviderName:  params.ProviderName,
		ProviderModel: params.ProviderModel,
		ID:            id,
		ProviderJobID: textValue(params.ProviderJobID),
		Status:        params.Status,
		ErrorCode:     textValue(params.ErrorCode),
		ErrorMessage:  textValue(params.ErrorMessage),
		CompletedAt:   timestamptzPointerValue(params.CompletedAt),
	})
	if err != nil {
		return fmt.Errorf("update image generation %s: %w", params.ID, err)
	}
	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

func mapImageGeneration(row dbsqlc.ImageGeneration) (ImageGeneration, error) {
	if !row.ID.Valid {
		return ImageGeneration{}, errors.New("image generation id is invalid")
	}
	if !row.SessionID.Valid {
		return ImageGeneration{}, errors.New("image generation session_id is invalid")
	}
	if !row.CreatedAt.Valid {
		return ImageGeneration{}, errors.New("image generation created_at is invalid")
	}

	return ImageGeneration{
		ID:                  row.ID.String(),
		SessionID:           row.SessionID.String(),
		Mode:                row.Mode,
		Prompt:              row.Prompt,
		ResolutionKey:       row.ResolutionKey,
		Width:               row.Width,
		Height:              row.Height,
		RequestedImageCount: row.RequestedImageCount,
		ProviderName:        row.ProviderName,
		ProviderModel:       row.ProviderModel,
		ProviderJobID:       nullableText(row.ProviderJobID),
		Status:              row.Status,
		ErrorCode:           nullableText(row.ErrorCode),
		ErrorMessage:        nullableText(row.ErrorMessage),
		CreatedAt:           row.CreatedAt.Time,
		CompletedAt:         nullableTime(row.CompletedAt),
	}, nil
}
