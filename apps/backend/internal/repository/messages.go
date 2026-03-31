package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/ivanlin/ulduar/apps/backend/internal/dbsqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type Message struct {
	ID        string
	SessionID string
	Role      string
	Content   json.RawMessage
	Status    string
	ModelName string
	CreatedAt time.Time
}

type CreateMessageParams struct {
	SessionID string
	Role      string
	Content   json.RawMessage
	Status    string
	ModelName string
}

type UpdateMessageStateParams struct {
	ID        string
	Status    string
	ModelName string
}

type UpdateMessageContentStateParams struct {
	ID        string
	Content   json.RawMessage
	Status    string
	ModelName string
}

type MessageRepository struct {
	queries *dbsqlc.Queries
}

func NewMessageRepository(db dbsqlc.DBTX) *MessageRepository {
	return &MessageRepository{
		queries: dbsqlc.New(db),
	}
}

func (r *MessageRepository) Create(ctx context.Context, params CreateMessageParams) (Message, error) {
	sessionID, err := parseUUID(params.SessionID)
	if err != nil {
		return Message{}, fmt.Errorf("parse session id: %w", err)
	}

	row, err := r.queries.CreateMessage(ctx, dbsqlc.CreateMessageParams{
		SessionID:   sessionID,
		Role:        params.Role,
		ContentJson: []byte(params.Content),
		Status:      params.Status,
		ModelName:   textValue(params.ModelName),
	})
	if err != nil {
		return Message{}, fmt.Errorf("create message: %w", err)
	}

	message, err := mapMessage(row)
	if err != nil {
		return Message{}, fmt.Errorf("map created message: %w", err)
	}

	return message, nil
}

func (r *MessageRepository) GetByID(ctx context.Context, messageID string) (Message, error) {
	id, err := parseUUID(messageID)
	if err != nil {
		return Message{}, fmt.Errorf("parse message id: %w", err)
	}

	row, err := r.queries.GetMessage(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Message{}, ErrNotFound
	}
	if err != nil {
		return Message{}, fmt.Errorf("get message %s: %w", messageID, err)
	}

	message, err := mapMessage(row)
	if err != nil {
		return Message{}, fmt.Errorf("map message %s: %w", messageID, err)
	}

	return message, nil
}

func (r *MessageRepository) ListBySession(ctx context.Context, sessionID string) ([]Message, error) {
	id, err := parseUUID(sessionID)
	if err != nil {
		return nil, fmt.Errorf("parse session id: %w", err)
	}

	rows, err := r.queries.ListMessagesBySession(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list messages for session %s: %w", sessionID, err)
	}

	messages := make([]Message, 0, len(rows))
	for _, row := range rows {
		message, err := mapMessage(row)
		if err != nil {
			return nil, fmt.Errorf("map message for session %s: %w", sessionID, err)
		}
		messages = append(messages, message)
	}

	return messages, nil
}

func (r *MessageRepository) UpdateState(ctx context.Context, params UpdateMessageStateParams) error {
	id, err := parseUUID(params.ID)
	if err != nil {
		return fmt.Errorf("parse message id: %w", err)
	}

	if err := r.queries.UpdateMessageStatusAndModel(ctx, dbsqlc.UpdateMessageStatusAndModelParams{
		ID:        id,
		Status:    params.Status,
		ModelName: textValue(params.ModelName),
	}); err != nil {
		return fmt.Errorf("update message %s: %w", params.ID, err)
	}

	return nil
}

func (r *MessageRepository) UpdateContentAndState(ctx context.Context, params UpdateMessageContentStateParams) error {
	id, err := parseUUID(params.ID)
	if err != nil {
		return fmt.Errorf("parse message id: %w", err)
	}

	rowsAffected, err := r.queries.UpdateMessageContentStatusAndModel(ctx, dbsqlc.UpdateMessageContentStatusAndModelParams{
		ID:          id,
		ContentJson: []byte(params.Content),
		Status:      params.Status,
		ModelName:   textValue(params.ModelName),
	})
	if err != nil {
		return fmt.Errorf("update message %s content/state: %w", params.ID, err)
	}
	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

func mapMessage(row dbsqlc.ChatMessage) (Message, error) {
	if !row.ID.Valid {
		return Message{}, errors.New("message id is invalid")
	}
	if !row.SessionID.Valid {
		return Message{}, errors.New("message session_id is invalid")
	}
	if !row.CreatedAt.Valid {
		return Message{}, errors.New("message created_at is invalid")
	}

	content := make(json.RawMessage, len(row.ContentJson))
	copy(content, row.ContentJson)

	return Message{
		ID:        row.ID.String(),
		SessionID: row.SessionID.String(),
		Role:      row.Role,
		Content:   content,
		Status:    row.Status,
		ModelName: nullableText(row.ModelName),
		CreatedAt: row.CreatedAt.Time,
	}, nil
}

func textValue(value string) pgtype.Text {
	if value == "" {
		return pgtype.Text{}
	}

	return pgtype.Text{
		String: value,
		Valid:  true,
	}
}

func nullableText(value pgtype.Text) string {
	if !value.Valid {
		return ""
	}

	return value.String
}
