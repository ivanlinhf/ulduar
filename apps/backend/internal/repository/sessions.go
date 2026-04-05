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

type Session struct {
	ID            string
	Status        string
	Title         string
	CreatedAt     time.Time
	LastMessageAt time.Time
}

type SessionRepository struct {
	queries *dbsqlc.Queries
}

func NewSessionRepository(db dbsqlc.DBTX) *SessionRepository {
	return &SessionRepository{
		queries: dbsqlc.New(db),
	}
}

func (r *SessionRepository) Create(ctx context.Context) (Session, error) {
	row, err := r.queries.CreateSession(ctx)
	if err != nil {
		return Session{}, fmt.Errorf("create session: %w", err)
	}

	session, err := mapSession(row)
	if err != nil {
		return Session{}, fmt.Errorf("map created session: %w", err)
	}

	return session, nil
}

func (r *SessionRepository) GetByID(ctx context.Context, sessionID string) (Session, error) {
	id, err := parseUUID(sessionID)
	if err != nil {
		return Session{}, fmt.Errorf("parse session id: %w", err)
	}

	row, err := r.queries.GetSession(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, fmt.Errorf("get session %s: %w", sessionID, err)
	}

	session, err := mapSession(row)
	if err != nil {
		return Session{}, fmt.Errorf("map session %s: %w", sessionID, err)
	}

	return session, nil
}

func (r *SessionRepository) TouchLastMessageAt(ctx context.Context, sessionID string) error {
	id, err := parseUUID(sessionID)
	if err != nil {
		return fmt.Errorf("parse session id: %w", err)
	}

	rowsAffected, err := r.queries.TouchSessionLastMessageAt(ctx, id)
	if err != nil {
		return fmt.Errorf("touch session %s: %w", sessionID, err)
	}
	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

func (r *SessionRepository) SetTitleIfEmpty(ctx context.Context, sessionID string, title string) error {
	id, err := parseUUID(sessionID)
	if err != nil {
		return fmt.Errorf("parse session id: %w", err)
	}

	_, err = r.queries.SetSessionTitleIfEmpty(ctx, dbsqlc.SetSessionTitleIfEmptyParams{
		ID:    id,
		Title: pgtype.Text{String: title, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("set session title %s: %w", sessionID, err)
	}

	return nil
}

func mapSession(row dbsqlc.ChatSession) (Session, error) {
	if !row.ID.Valid {
		return Session{}, errors.New("session id is invalid")
	}
	if !row.CreatedAt.Valid {
		return Session{}, errors.New("session created_at is invalid")
	}
	if !row.LastMessageAt.Valid {
		return Session{}, errors.New("session last_message_at is invalid")
	}

	return Session{
		ID:            row.ID.String(),
		Status:        row.Status,
		Title:         row.Title.String,
		CreatedAt:     row.CreatedAt.Time,
		LastMessageAt: row.LastMessageAt.Time,
	}, nil
}

func parseUUID(value string) (pgtype.UUID, error) {
	var uuid pgtype.UUID
	if err := uuid.Scan(value); err != nil {
		return pgtype.UUID{}, err
	}

	return uuid, nil
}
