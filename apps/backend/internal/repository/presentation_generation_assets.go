package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ivanlin/ulduar/apps/backend/internal/dbsqlc"
	"github.com/jackc/pgx/v5"
)

type PresentationGenerationAsset struct {
	ID            string
	GenerationID  string
	Role          string
	AssetRef      string
	SourceType    string
	SourceAssetID string
	SourceRef     string
	SortOrder     int64
	BlobPath      string
	MediaType     string
	Filename      string
	SizeBytes     int64
	Sha256        string
	CreatedAt     time.Time
}

type CreatePresentationGenerationAssetParams struct {
	GenerationID  string
	Role          string
	AssetRef      string
	SourceType    string
	SourceAssetID string
	SourceRef     string
	SortOrder     int64
	BlobPath      string
	MediaType     string
	Filename      string
	SizeBytes     int64
	Sha256        string
}

type PresentationGenerationAssetRepository struct {
	queries *dbsqlc.Queries
}

func NewPresentationGenerationAssetRepository(db dbsqlc.DBTX) *PresentationGenerationAssetRepository {
	return &PresentationGenerationAssetRepository{
		queries: dbsqlc.New(db),
	}
}

func (r *PresentationGenerationAssetRepository) Create(ctx context.Context, params CreatePresentationGenerationAssetParams) (PresentationGenerationAsset, error) {
	generationID, err := parseUUID(params.GenerationID)
	if err != nil {
		return PresentationGenerationAsset{}, fmt.Errorf("parse generation id: %w", err)
	}
	sourceAssetID, err := parseOptionalUUID(params.SourceAssetID)
	if err != nil {
		return PresentationGenerationAsset{}, fmt.Errorf("parse source asset id: %w", err)
	}

	row, err := r.queries.CreatePresentationGenerationAsset(ctx, dbsqlc.CreatePresentationGenerationAssetParams{
		GenerationID:  generationID,
		Role:          params.Role,
		AssetRef:      textValue(params.AssetRef),
		SourceType:    textValue(params.SourceType),
		SourceAssetID: sourceAssetID,
		SourceRef:     textValue(params.SourceRef),
		SortOrder:     params.SortOrder,
		BlobPath:      params.BlobPath,
		MediaType:     params.MediaType,
		Filename:      params.Filename,
		SizeBytes:     params.SizeBytes,
		Sha256:        params.Sha256,
	})
	if err != nil {
		return PresentationGenerationAsset{}, fmt.Errorf("create presentation generation asset: %w", err)
	}

	asset, err := mapPresentationGenerationAsset(row)
	if err != nil {
		return PresentationGenerationAsset{}, fmt.Errorf("map created presentation generation asset: %w", err)
	}

	return asset, nil
}

func (r *PresentationGenerationAssetRepository) GetByID(ctx context.Context, assetID string) (PresentationGenerationAsset, error) {
	id, err := parseUUID(assetID)
	if err != nil {
		return PresentationGenerationAsset{}, fmt.Errorf("parse presentation generation asset id: %w", err)
	}

	row, err := r.queries.GetPresentationGenerationAsset(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return PresentationGenerationAsset{}, ErrNotFound
	}
	if err != nil {
		return PresentationGenerationAsset{}, fmt.Errorf("get presentation generation asset %s: %w", assetID, err)
	}

	asset, err := mapPresentationGenerationAsset(row)
	if err != nil {
		return PresentationGenerationAsset{}, fmt.Errorf("map presentation generation asset %s: %w", assetID, err)
	}

	return asset, nil
}

func (r *PresentationGenerationAssetRepository) GetByIDAndSession(ctx context.Context, assetID string, sessionID string) (PresentationGenerationAsset, error) {
	assetUUID, err := parseUUID(assetID)
	if err != nil {
		return PresentationGenerationAsset{}, fmt.Errorf("parse presentation generation asset id: %w", err)
	}

	sessionUUID, err := parseUUID(sessionID)
	if err != nil {
		return PresentationGenerationAsset{}, fmt.Errorf("parse session id: %w", err)
	}

	row, err := r.queries.GetPresentationGenerationAssetBySession(ctx, dbsqlc.GetPresentationGenerationAssetBySessionParams{
		ID:        assetUUID,
		SessionID: sessionUUID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return PresentationGenerationAsset{}, ErrNotFound
	}
	if err != nil {
		return PresentationGenerationAsset{}, fmt.Errorf("get presentation generation asset %s for session %s: %w", assetID, sessionID, err)
	}

	asset, err := mapPresentationGenerationAsset(row)
	if err != nil {
		return PresentationGenerationAsset{}, fmt.Errorf("map presentation generation asset %s for session %s: %w", assetID, sessionID, err)
	}

	return asset, nil
}

func (r *PresentationGenerationAssetRepository) ListByGeneration(ctx context.Context, generationID string) ([]PresentationGenerationAsset, error) {
	id, err := parseUUID(generationID)
	if err != nil {
		return nil, fmt.Errorf("parse generation id: %w", err)
	}

	rows, err := r.queries.ListPresentationGenerationAssetsByGeneration(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list presentation generation assets for generation %s: %w", generationID, err)
	}

	assets := make([]PresentationGenerationAsset, 0, len(rows))
	for _, row := range rows {
		asset, err := mapPresentationGenerationAsset(row)
		if err != nil {
			return nil, fmt.Errorf("map presentation generation asset for generation %s: %w", generationID, err)
		}
		assets = append(assets, asset)
	}

	return assets, nil
}

func (r *PresentationGenerationAssetRepository) ListByGenerationAndSession(ctx context.Context, generationID string, sessionID string) ([]PresentationGenerationAsset, error) {
	generationUUID, err := parseUUID(generationID)
	if err != nil {
		return nil, fmt.Errorf("parse generation id: %w", err)
	}

	sessionUUID, err := parseUUID(sessionID)
	if err != nil {
		return nil, fmt.Errorf("parse session id: %w", err)
	}

	rows, err := r.queries.ListPresentationGenerationAssetsByGenerationAndSession(ctx, dbsqlc.ListPresentationGenerationAssetsByGenerationAndSessionParams{
		GenerationID: generationUUID,
		SessionID:    sessionUUID,
	})
	if err != nil {
		return nil, fmt.Errorf("list presentation generation assets for generation %s in session %s: %w", generationID, sessionID, err)
	}

	assets := make([]PresentationGenerationAsset, 0, len(rows))
	for _, row := range rows {
		asset, err := mapPresentationGenerationAsset(row)
		if err != nil {
			return nil, fmt.Errorf("map presentation generation asset for generation %s in session %s: %w", generationID, sessionID, err)
		}
		assets = append(assets, asset)
	}

	return assets, nil
}

func mapPresentationGenerationAsset(row dbsqlc.PresentationGenerationAsset) (PresentationGenerationAsset, error) {
	if !row.ID.Valid {
		return PresentationGenerationAsset{}, errors.New("presentation generation asset id is invalid")
	}
	if !row.GenerationID.Valid {
		return PresentationGenerationAsset{}, errors.New("presentation generation asset generation_id is invalid")
	}
	if !row.CreatedAt.Valid {
		return PresentationGenerationAsset{}, errors.New("presentation generation asset created_at is invalid")
	}

	return PresentationGenerationAsset{
		ID:            row.ID.String(),
		GenerationID:  row.GenerationID.String(),
		Role:          row.Role,
		AssetRef:      nullableText(row.AssetRef),
		SourceType:    nullableText(row.SourceType),
		SourceAssetID: nullableUUID(row.SourceAssetID),
		SourceRef:     nullableText(row.SourceRef),
		SortOrder:     row.SortOrder,
		BlobPath:      row.BlobPath,
		MediaType:     row.MediaType,
		Filename:      row.Filename,
		SizeBytes:     row.SizeBytes,
		Sha256:        row.Sha256,
		CreatedAt:     row.CreatedAt.Time,
	}, nil
}
