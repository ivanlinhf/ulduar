package repository

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/ivanlin/ulduar/apps/backend/internal/dbsqlc"
	"github.com/jackc/pgx/v5"
)

type PresentationGeneration struct {
	ID            string
	SessionID     string
	Prompt        string
	DialectJSON   []byte
	ProviderName  string
	ProviderModel string
	ProviderJobID string
	Status        string
	ErrorCode     string
	ErrorMessage  string
	CreatedAt     time.Time
	StartedAt     *time.Time
	CompletedAt   *time.Time
}

type CreatePresentationGenerationParams struct {
	SessionID     string
	Prompt        string
	DialectJSON   []byte
	ProviderName  string
	ProviderModel string
	ProviderJobID string
	Status        string
	ErrorCode     string
	ErrorMessage  string
	CompletedAt   *time.Time
}

type UpdatePresentationGenerationStateParams struct {
	ProviderName  string
	ProviderModel string
	ID            string
	ProviderJobID string
	Status        string
	ErrorCode     string
	ErrorMessage  string
	CompletedAt   *time.Time
	DialectJSON   []byte
}

type ClaimPendingPresentationGenerationParams struct {
	ID            string
	ProviderName  string
	ProviderModel string
}

type PresentationGenerationRepository struct {
	queries *dbsqlc.Queries
}

func NewPresentationGenerationRepository(db dbsqlc.DBTX) *PresentationGenerationRepository {
	return &PresentationGenerationRepository{
		queries: dbsqlc.New(db),
	}
}

func (r *PresentationGenerationRepository) Create(ctx context.Context, params CreatePresentationGenerationParams) (PresentationGeneration, error) {
	sessionID, err := parseUUID(params.SessionID)
	if err != nil {
		return PresentationGeneration{}, fmt.Errorf("parse session id: %w", err)
	}

	row, err := r.queries.CreatePresentationGeneration(ctx, dbsqlc.CreatePresentationGenerationParams{
		SessionID:     sessionID,
		Prompt:        params.Prompt,
		DialectJson:   slices.Clone(params.DialectJSON),
		ProviderName:  params.ProviderName,
		ProviderModel: params.ProviderModel,
		ProviderJobID: textValue(params.ProviderJobID),
		Status:        params.Status,
		ErrorCode:     textValue(params.ErrorCode),
		ErrorMessage:  textValue(params.ErrorMessage),
		CompletedAt:   timestamptzPointerValue(params.CompletedAt),
	})
	if err != nil {
		return PresentationGeneration{}, fmt.Errorf("create presentation generation: %w", err)
	}

	generation, err := mapPresentationGeneration(row)
	if err != nil {
		return PresentationGeneration{}, fmt.Errorf("map created presentation generation: %w", err)
	}

	return generation, nil
}

func (r *PresentationGenerationRepository) GetByID(ctx context.Context, generationID string) (PresentationGeneration, error) {
	id, err := parseUUID(generationID)
	if err != nil {
		return PresentationGeneration{}, fmt.Errorf("parse presentation generation id: %w", err)
	}

	row, err := r.queries.GetPresentationGeneration(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return PresentationGeneration{}, ErrNotFound
	}
	if err != nil {
		return PresentationGeneration{}, fmt.Errorf("get presentation generation %s: %w", generationID, err)
	}

	generation, err := mapPresentationGeneration(row)
	if err != nil {
		return PresentationGeneration{}, fmt.Errorf("map presentation generation %s: %w", generationID, err)
	}

	return generation, nil
}

func (r *PresentationGenerationRepository) GetByIDAndSession(ctx context.Context, generationID string, sessionID string) (PresentationGeneration, error) {
	generationUUID, err := parseUUID(generationID)
	if err != nil {
		return PresentationGeneration{}, fmt.Errorf("parse presentation generation id: %w", err)
	}

	sessionUUID, err := parseUUID(sessionID)
	if err != nil {
		return PresentationGeneration{}, fmt.Errorf("parse session id: %w", err)
	}

	row, err := r.queries.GetPresentationGenerationBySession(ctx, dbsqlc.GetPresentationGenerationBySessionParams{
		ID:        generationUUID,
		SessionID: sessionUUID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return PresentationGeneration{}, ErrNotFound
	}
	if err != nil {
		return PresentationGeneration{}, fmt.Errorf("get presentation generation %s for session %s: %w", generationID, sessionID, err)
	}

	generation, err := mapPresentationGeneration(row)
	if err != nil {
		return PresentationGeneration{}, fmt.Errorf("map presentation generation %s for session %s: %w", generationID, sessionID, err)
	}

	return generation, nil
}

func (r *PresentationGenerationRepository) ListBySession(ctx context.Context, sessionID string) ([]PresentationGeneration, error) {
	id, err := parseUUID(sessionID)
	if err != nil {
		return nil, fmt.Errorf("parse session id: %w", err)
	}

	rows, err := r.queries.ListPresentationGenerationsBySession(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list presentation generations for session %s: %w", sessionID, err)
	}

	generations := make([]PresentationGeneration, 0, len(rows))
	for _, row := range rows {
		generation, err := mapPresentationGeneration(row)
		if err != nil {
			return nil, fmt.Errorf("map presentation generation for session %s: %w", sessionID, err)
		}
		generations = append(generations, generation)
	}

	return generations, nil
}

func (r *PresentationGenerationRepository) ClaimPending(ctx context.Context, params ClaimPendingPresentationGenerationParams) (bool, error) {
	id, err := parseUUID(params.ID)
	if err != nil {
		return false, fmt.Errorf("parse presentation generation id: %w", err)
	}

	rowsAffected, err := r.queries.ClaimPendingPresentationGeneration(ctx, dbsqlc.ClaimPendingPresentationGenerationParams{
		ID:            id,
		ProviderName:  params.ProviderName,
		ProviderModel: params.ProviderModel,
	})
	if err != nil {
		return false, fmt.Errorf("claim pending presentation generation %s: %w", params.ID, err)
	}

	return rowsAffected > 0, nil
}

func (r *PresentationGenerationRepository) LockForUpdate(ctx context.Context, generationID string) (PresentationGeneration, error) {
	id, err := parseUUID(generationID)
	if err != nil {
		return PresentationGeneration{}, fmt.Errorf("parse presentation generation id: %w", err)
	}

	row, err := r.queries.LockPresentationGenerationForUpdate(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return PresentationGeneration{}, ErrNotFound
	}
	if err != nil {
		return PresentationGeneration{}, fmt.Errorf("lock presentation generation %s: %w", generationID, err)
	}

	generation, err := mapPresentationGeneration(row)
	if err != nil {
		return PresentationGeneration{}, fmt.Errorf("map locked presentation generation %s: %w", generationID, err)
	}

	return generation, nil
}

func (r *PresentationGenerationRepository) UpdateState(ctx context.Context, params UpdatePresentationGenerationStateParams) error {
	id, err := parseUUID(params.ID)
	if err != nil {
		return fmt.Errorf("parse presentation generation id: %w", err)
	}

	rowsAffected, err := r.queries.UpdatePresentationGenerationState(ctx, dbsqlc.UpdatePresentationGenerationStateParams{
		ProviderName:  params.ProviderName,
		ProviderModel: params.ProviderModel,
		ID:            id,
		ProviderJobID: textValue(params.ProviderJobID),
		Status:        params.Status,
		ErrorCode:     textValue(params.ErrorCode),
		ErrorMessage:  textValue(params.ErrorMessage),
		CompletedAt:   timestamptzPointerValue(params.CompletedAt),
		DialectJson:   slices.Clone(params.DialectJSON),
	})
	if err != nil {
		return fmt.Errorf("update presentation generation %s: %w", params.ID, err)
	}
	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

func mapPresentationGeneration(row dbsqlc.PresentationGeneration) (PresentationGeneration, error) {
	if !row.ID.Valid {
		return PresentationGeneration{}, errors.New("presentation generation id is invalid")
	}
	if !row.SessionID.Valid {
		return PresentationGeneration{}, errors.New("presentation generation session_id is invalid")
	}
	if !row.CreatedAt.Valid {
		return PresentationGeneration{}, errors.New("presentation generation created_at is invalid")
	}

	return PresentationGeneration{
		ID:            row.ID.String(),
		SessionID:     row.SessionID.String(),
		Prompt:        row.Prompt,
		DialectJSON:   slices.Clone(row.DialectJson),
		ProviderName:  row.ProviderName,
		ProviderModel: row.ProviderModel,
		ProviderJobID: nullableText(row.ProviderJobID),
		Status:        row.Status,
		ErrorCode:     nullableText(row.ErrorCode),
		ErrorMessage:  nullableText(row.ErrorMessage),
		CreatedAt:     row.CreatedAt.Time,
		StartedAt:     nullableTime(row.StartedAt),
		CompletedAt:   nullableTime(row.CompletedAt),
	}, nil
}
