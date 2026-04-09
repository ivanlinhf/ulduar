package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ivanlin/ulduar/apps/backend/internal/dbsqlc"
	"github.com/jackc/pgx/v5"
)

type ImageGenerationAsset struct {
	ID           string
	GenerationID string
	Role         string
	SortOrder    int64
	BlobPath     string
	MediaType    string
	Filename     string
	SizeBytes    int64
	Sha256       string
	Width        *int64
	Height       *int64
	CreatedAt    time.Time
}

type CreateImageGenerationAssetParams struct {
	GenerationID string
	Role         string
	SortOrder    int64
	BlobPath     string
	MediaType    string
	Filename     string
	SizeBytes    int64
	Sha256       string
	Width        *int64
	Height       *int64
}

type ImageGenerationAssetRepository struct {
	queries *dbsqlc.Queries
}

func NewImageGenerationAssetRepository(db dbsqlc.DBTX) *ImageGenerationAssetRepository {
	return &ImageGenerationAssetRepository{
		queries: dbsqlc.New(db),
	}
}

func (r *ImageGenerationAssetRepository) Create(ctx context.Context, params CreateImageGenerationAssetParams) (ImageGenerationAsset, error) {
	generationID, err := parseUUID(params.GenerationID)
	if err != nil {
		return ImageGenerationAsset{}, fmt.Errorf("parse generation id: %w", err)
	}

	row, err := r.queries.CreateImageGenerationAsset(ctx, dbsqlc.CreateImageGenerationAssetParams{
		GenerationID: generationID,
		Role:         params.Role,
		SortOrder:    params.SortOrder,
		BlobPath:     params.BlobPath,
		MediaType:    params.MediaType,
		Filename:     params.Filename,
		SizeBytes:    params.SizeBytes,
		Sha256:       params.Sha256,
		Width:        int8PointerValue(params.Width),
		Height:       int8PointerValue(params.Height),
	})
	if err != nil {
		return ImageGenerationAsset{}, fmt.Errorf("create image generation asset: %w", err)
	}

	asset, err := mapImageGenerationAsset(row)
	if err != nil {
		return ImageGenerationAsset{}, fmt.Errorf("map created image generation asset: %w", err)
	}

	return asset, nil
}

func (r *ImageGenerationAssetRepository) GetByID(ctx context.Context, assetID string) (ImageGenerationAsset, error) {
	id, err := parseUUID(assetID)
	if err != nil {
		return ImageGenerationAsset{}, fmt.Errorf("parse image generation asset id: %w", err)
	}

	row, err := r.queries.GetImageGenerationAsset(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ImageGenerationAsset{}, ErrNotFound
	}
	if err != nil {
		return ImageGenerationAsset{}, fmt.Errorf("get image generation asset %s: %w", assetID, err)
	}

	asset, err := mapImageGenerationAsset(row)
	if err != nil {
		return ImageGenerationAsset{}, fmt.Errorf("map image generation asset %s: %w", assetID, err)
	}

	return asset, nil
}

func (r *ImageGenerationAssetRepository) GetByIDAndSession(ctx context.Context, assetID string, sessionID string) (ImageGenerationAsset, error) {
	assetUUID, err := parseUUID(assetID)
	if err != nil {
		return ImageGenerationAsset{}, fmt.Errorf("parse image generation asset id: %w", err)
	}

	sessionUUID, err := parseUUID(sessionID)
	if err != nil {
		return ImageGenerationAsset{}, fmt.Errorf("parse session id: %w", err)
	}

	row, err := r.queries.GetImageGenerationAssetBySession(ctx, dbsqlc.GetImageGenerationAssetBySessionParams{
		ID:        assetUUID,
		SessionID: sessionUUID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ImageGenerationAsset{}, ErrNotFound
	}
	if err != nil {
		return ImageGenerationAsset{}, fmt.Errorf("get image generation asset %s for session %s: %w", assetID, sessionID, err)
	}

	asset, err := mapImageGenerationAsset(row)
	if err != nil {
		return ImageGenerationAsset{}, fmt.Errorf("map image generation asset %s for session %s: %w", assetID, sessionID, err)
	}

	return asset, nil
}

func (r *ImageGenerationAssetRepository) ListByGeneration(ctx context.Context, generationID string) ([]ImageGenerationAsset, error) {
	id, err := parseUUID(generationID)
	if err != nil {
		return nil, fmt.Errorf("parse generation id: %w", err)
	}

	rows, err := r.queries.ListImageGenerationAssetsByGeneration(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list image generation assets for generation %s: %w", generationID, err)
	}

	assets := make([]ImageGenerationAsset, 0, len(rows))
	for _, row := range rows {
		asset, err := mapImageGenerationAsset(row)
		if err != nil {
			return nil, fmt.Errorf("map image generation asset for generation %s: %w", generationID, err)
		}
		assets = append(assets, asset)
	}

	return assets, nil
}

func (r *ImageGenerationAssetRepository) ListByGenerationAndSession(ctx context.Context, generationID string, sessionID string) ([]ImageGenerationAsset, error) {
	generationUUID, err := parseUUID(generationID)
	if err != nil {
		return nil, fmt.Errorf("parse generation id: %w", err)
	}

	sessionUUID, err := parseUUID(sessionID)
	if err != nil {
		return nil, fmt.Errorf("parse session id: %w", err)
	}

	rows, err := r.queries.ListImageGenerationAssetsByGenerationAndSession(ctx, dbsqlc.ListImageGenerationAssetsByGenerationAndSessionParams{
		GenerationID: generationUUID,
		SessionID:    sessionUUID,
	})
	if err != nil {
		return nil, fmt.Errorf("list image generation assets for generation %s in session %s: %w", generationID, sessionID, err)
	}

	assets := make([]ImageGenerationAsset, 0, len(rows))
	for _, row := range rows {
		asset, err := mapImageGenerationAsset(row)
		if err != nil {
			return nil, fmt.Errorf("map image generation asset for generation %s in session %s: %w", generationID, sessionID, err)
		}
		assets = append(assets, asset)
	}

	return assets, nil
}

func mapImageGenerationAsset(row dbsqlc.ImageGenerationAsset) (ImageGenerationAsset, error) {
	if !row.ID.Valid {
		return ImageGenerationAsset{}, errors.New("image generation asset id is invalid")
	}
	if !row.GenerationID.Valid {
		return ImageGenerationAsset{}, errors.New("image generation asset generation_id is invalid")
	}
	if !row.CreatedAt.Valid {
		return ImageGenerationAsset{}, errors.New("image generation asset created_at is invalid")
	}

	return ImageGenerationAsset{
		ID:           row.ID.String(),
		GenerationID: row.GenerationID.String(),
		Role:         row.Role,
		SortOrder:    row.SortOrder,
		BlobPath:     row.BlobPath,
		MediaType:    row.MediaType,
		Filename:     row.Filename,
		SizeBytes:    row.SizeBytes,
		Sha256:       row.Sha256,
		Width:        nullableInt8(row.Width),
		Height:       nullableInt8(row.Height),
		CreatedAt:    row.CreatedAt.Time,
	}, nil
}
