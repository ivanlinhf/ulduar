package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ivanlin/ulduar/apps/backend/internal/dbsqlc"
)

type Attachment struct {
	ID             string
	SessionID      string
	MessageID      string
	BlobPath       string
	MediaType      string
	Filename       string
	SizeBytes      int64
	Sha256         string
	ProviderFileID string
	CreatedAt      time.Time
}

type CreateAttachmentParams struct {
	SessionID      string
	MessageID      string
	BlobPath       string
	MediaType      string
	Filename       string
	SizeBytes      int64
	Sha256         string
	ProviderFileID string
}

type AttachmentRepository struct {
	queries *dbsqlc.Queries
}

func NewAttachmentRepository(db dbsqlc.DBTX) *AttachmentRepository {
	return &AttachmentRepository{
		queries: dbsqlc.New(db),
	}
}

func (r *AttachmentRepository) Create(ctx context.Context, params CreateAttachmentParams) (Attachment, error) {
	sessionID, err := parseUUID(params.SessionID)
	if err != nil {
		return Attachment{}, fmt.Errorf("parse session id: %w", err)
	}

	messageID, err := parseUUID(params.MessageID)
	if err != nil {
		return Attachment{}, fmt.Errorf("parse message id: %w", err)
	}

	row, err := r.queries.CreateAttachment(ctx, dbsqlc.CreateAttachmentParams{
		SessionID:      sessionID,
		MessageID:      messageID,
		BlobPath:       params.BlobPath,
		MediaType:      params.MediaType,
		Filename:       params.Filename,
		SizeBytes:      params.SizeBytes,
		Sha256:         params.Sha256,
		ProviderFileID: textValue(params.ProviderFileID),
	})
	if err != nil {
		return Attachment{}, fmt.Errorf("create attachment: %w", err)
	}

	attachment, err := mapAttachment(row)
	if err != nil {
		return Attachment{}, fmt.Errorf("map created attachment: %w", err)
	}

	return attachment, nil
}

func (r *AttachmentRepository) ListByMessage(ctx context.Context, messageID string) ([]Attachment, error) {
	id, err := parseUUID(messageID)
	if err != nil {
		return nil, fmt.Errorf("parse message id: %w", err)
	}

	rows, err := r.queries.ListAttachmentsByMessage(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list attachments for message %s: %w", messageID, err)
	}

	attachments := make([]Attachment, 0, len(rows))
	for _, row := range rows {
		attachment, err := mapAttachment(row)
		if err != nil {
			return nil, fmt.Errorf("map attachment for message %s: %w", messageID, err)
		}
		attachments = append(attachments, attachment)
	}

	return attachments, nil
}

func mapAttachment(row dbsqlc.ChatAttachment) (Attachment, error) {
	if !row.ID.Valid {
		return Attachment{}, errors.New("attachment id is invalid")
	}
	if !row.SessionID.Valid {
		return Attachment{}, errors.New("attachment session_id is invalid")
	}
	if !row.MessageID.Valid {
		return Attachment{}, errors.New("attachment message_id is invalid")
	}
	if !row.CreatedAt.Valid {
		return Attachment{}, errors.New("attachment created_at is invalid")
	}

	return Attachment{
		ID:             row.ID.String(),
		SessionID:      row.SessionID.String(),
		MessageID:      row.MessageID.String(),
		BlobPath:       row.BlobPath,
		MediaType:      row.MediaType,
		Filename:       row.Filename,
		SizeBytes:      row.SizeBytes,
		Sha256:         row.Sha256,
		ProviderFileID: nullableText(row.ProviderFileID),
		CreatedAt:      row.CreatedAt.Time,
	}, nil
}
