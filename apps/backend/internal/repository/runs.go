package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ivanlin/ulduar/apps/backend/internal/dbsqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type Run struct {
	ID                 string
	SessionID          string
	UserMessageID      string
	AssistantMessageID string
	ProviderResponseID string
	InputTokens        *int64
	OutputTokens       *int64
	TotalTokens        *int64
	Status             string
	ErrorCode          string
	StartedAt          time.Time
	CompletedAt        *time.Time
}

type CreateRunParams struct {
	SessionID          string
	UserMessageID      string
	AssistantMessageID string
	ProviderResponseID string
	InputTokens        *int64
	OutputTokens       *int64
	TotalTokens        *int64
	Status             string
	ErrorCode          string
	StartedAt          time.Time
	CompletedAt        *time.Time
}

type UpdateRunStateParams struct {
	ID                 string
	AssistantMessageID string
	ProviderResponseID string
	InputTokens        *int64
	OutputTokens       *int64
	TotalTokens        *int64
	Status             string
	ErrorCode          string
	CompletedAt        *time.Time
}

type RunRepository struct {
	queries *dbsqlc.Queries
}

func NewRunRepository(db dbsqlc.DBTX) *RunRepository {
	return &RunRepository{
		queries: dbsqlc.New(db),
	}
}

func (r *RunRepository) Create(ctx context.Context, params CreateRunParams) (Run, error) {
	sessionID, err := parseUUID(params.SessionID)
	if err != nil {
		return Run{}, fmt.Errorf("parse session id: %w", err)
	}

	userMessageID, err := parseUUID(params.UserMessageID)
	if err != nil {
		return Run{}, fmt.Errorf("parse user message id: %w", err)
	}

	assistantMessageID, err := parseOptionalUUID(params.AssistantMessageID)
	if err != nil {
		return Run{}, fmt.Errorf("parse assistant message id: %w", err)
	}

	row, err := r.queries.CreateRun(ctx, dbsqlc.CreateRunParams{
		SessionID:          sessionID,
		UserMessageID:      userMessageID,
		AssistantMessageID: assistantMessageID,
		ProviderResponseID: textValue(params.ProviderResponseID),
		InputTokens:        int8PointerValue(params.InputTokens),
		OutputTokens:       int8PointerValue(params.OutputTokens),
		TotalTokens:        int8PointerValue(params.TotalTokens),
		Status:             params.Status,
		ErrorCode:          textValue(params.ErrorCode),
		StartedAt:          timestamptzValue(params.StartedAt),
		CompletedAt:        timestamptzPointerValue(params.CompletedAt),
	})
	if err != nil {
		return Run{}, fmt.Errorf("create run: %w", err)
	}

	run, err := mapRun(row)
	if err != nil {
		return Run{}, fmt.Errorf("map created run: %w", err)
	}

	return run, nil
}

func (r *RunRepository) ListBySession(ctx context.Context, sessionID string) ([]Run, error) {
	id, err := parseUUID(sessionID)
	if err != nil {
		return nil, fmt.Errorf("parse session id: %w", err)
	}

	rows, err := r.queries.ListRunsBySession(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list runs for session %s: %w", sessionID, err)
	}

	runs := make([]Run, 0, len(rows))
	for _, row := range rows {
		run, err := mapRun(row)
		if err != nil {
			return nil, fmt.Errorf("map run for session %s: %w", sessionID, err)
		}
		runs = append(runs, run)
	}

	return runs, nil
}

func (r *RunRepository) GetByID(ctx context.Context, runID string) (Run, error) {
	id, err := parseUUID(runID)
	if err != nil {
		return Run{}, fmt.Errorf("parse run id: %w", err)
	}

	row, err := r.queries.GetRun(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Run{}, ErrNotFound
	}
	if err != nil {
		return Run{}, fmt.Errorf("get run %s: %w", runID, err)
	}

	run, err := mapRun(row)
	if err != nil {
		return Run{}, fmt.Errorf("map run %s: %w", runID, err)
	}

	return run, nil
}

func (r *RunRepository) UpdateState(ctx context.Context, params UpdateRunStateParams) error {
	id, err := parseUUID(params.ID)
	if err != nil {
		return fmt.Errorf("parse run id: %w", err)
	}

	assistantMessageID, err := parseOptionalUUID(params.AssistantMessageID)
	if err != nil {
		return fmt.Errorf("parse assistant message id: %w", err)
	}

	rowsAffected, err := r.queries.UpdateRunState(ctx, dbsqlc.UpdateRunStateParams{
		ID:                 id,
		AssistantMessageID: assistantMessageID,
		ProviderResponseID: textValue(params.ProviderResponseID),
		InputTokens:        int8PointerValue(params.InputTokens),
		OutputTokens:       int8PointerValue(params.OutputTokens),
		TotalTokens:        int8PointerValue(params.TotalTokens),
		Status:             params.Status,
		ErrorCode:          textValue(params.ErrorCode),
		CompletedAt:        timestamptzPointerValue(params.CompletedAt),
	})
	if err != nil {
		return fmt.Errorf("update run %s: %w", params.ID, err)
	}
	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

func mapRun(row dbsqlc.ChatRun) (Run, error) {
	if !row.ID.Valid {
		return Run{}, errors.New("run id is invalid")
	}
	if !row.SessionID.Valid {
		return Run{}, errors.New("run session_id is invalid")
	}
	if !row.UserMessageID.Valid {
		return Run{}, errors.New("run user_message_id is invalid")
	}
	if !row.StartedAt.Valid {
		return Run{}, errors.New("run started_at is invalid")
	}

	return Run{
		ID:                 row.ID.String(),
		SessionID:          row.SessionID.String(),
		UserMessageID:      row.UserMessageID.String(),
		AssistantMessageID: nullableUUID(row.AssistantMessageID),
		ProviderResponseID: nullableText(row.ProviderResponseID),
		InputTokens:        nullableInt8(row.InputTokens),
		OutputTokens:       nullableInt8(row.OutputTokens),
		TotalTokens:        nullableInt8(row.TotalTokens),
		Status:             row.Status,
		ErrorCode:          nullableText(row.ErrorCode),
		StartedAt:          row.StartedAt.Time,
		CompletedAt:        nullableTime(row.CompletedAt),
	}, nil
}

func parseOptionalUUID(value string) (pgtype.UUID, error) {
	if value == "" {
		return pgtype.UUID{}, nil
	}

	return parseUUID(value)
}

func nullableUUID(value pgtype.UUID) string {
	if !value.Valid {
		return ""
	}

	return value.String()
}

func timestamptzValue(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{
		Time:  value,
		Valid: true,
	}
}

func timestamptzPointerValue(value *time.Time) pgtype.Timestamptz {
	if value == nil {
		return pgtype.Timestamptz{}
	}

	return pgtype.Timestamptz{
		Time:  *value,
		Valid: true,
	}
}

func nullableTime(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}

	t := value.Time
	return &t
}

func int8PointerValue(value *int64) pgtype.Int8 {
	if value == nil {
		return pgtype.Int8{}
	}

	return pgtype.Int8{
		Int64: *value,
		Valid: true,
	}
}

func nullableInt8(value pgtype.Int8) *int64 {
	if !value.Valid {
		return nil
	}

	v := value.Int64
	return &v
}
