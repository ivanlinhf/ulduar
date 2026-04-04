package repository

import (
	"testing"
	"time"

	"github.com/ivanlin/ulduar/apps/backend/internal/dbsqlc"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestMapSession(t *testing.T) {
	now := time.Date(2026, 3, 31, 10, 0, 0, 0, time.UTC)
	row := dbsqlc.ChatSession{
		ID:            mustUUID(t, "11111111-1111-1111-1111-111111111111"),
		Status:        "active",
		CreatedAt:     mustTime(now),
		LastMessageAt: mustTime(now.Add(2 * time.Minute)),
	}

	session, err := mapSession(row)
	if err != nil {
		t.Fatalf("mapSession() error = %v", err)
	}

	if session.ID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("session.ID = %q", session.ID)
	}
	if session.LastMessageAt != now.Add(2*time.Minute) {
		t.Fatalf("session.LastMessageAt = %v", session.LastMessageAt)
	}
}

func TestMapSessionRejectsInvalidID(t *testing.T) {
	_, err := mapSession(dbsqlc.ChatSession{
		Status:        "active",
		CreatedAt:     mustTime(time.Now().UTC()),
		LastMessageAt: mustTime(time.Now().UTC()),
	})
	if err == nil || err.Error() != "session id is invalid" {
		t.Fatalf("mapSession() error = %v, want invalid id", err)
	}
}

func TestMapMessage(t *testing.T) {
	now := time.Date(2026, 3, 31, 10, 5, 0, 0, time.UTC)
	row := dbsqlc.ChatMessage{
		ID:          mustUUID(t, "22222222-2222-2222-2222-222222222222"),
		SessionID:   mustUUID(t, "11111111-1111-1111-1111-111111111111"),
		Role:        "assistant",
		ContentJson: []byte(`{"parts":[{"type":"text","text":"hello"}]}`),
		Status:      "completed",
		ModelName:   textValue("gpt-5"),
		CreatedAt:   mustTime(now),
	}

	message, err := mapMessage(row)
	if err != nil {
		t.Fatalf("mapMessage() error = %v", err)
	}

	if message.ID != "22222222-2222-2222-2222-222222222222" {
		t.Fatalf("message.ID = %q", message.ID)
	}
	if message.SessionID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("message.SessionID = %q", message.SessionID)
	}
	if string(message.Content) != `{"parts":[{"type":"text","text":"hello"}]}` {
		t.Fatalf("message.Content = %s", string(message.Content))
	}
	if message.ModelName != "gpt-5" {
		t.Fatalf("message.ModelName = %q", message.ModelName)
	}
}

func TestMapAttachment(t *testing.T) {
	now := time.Date(2026, 3, 31, 10, 10, 0, 0, time.UTC)
	row := dbsqlc.ChatAttachment{
		ID:             mustUUID(t, "33333333-3333-3333-3333-333333333333"),
		SessionID:      mustUUID(t, "11111111-1111-1111-1111-111111111111"),
		MessageID:      mustUUID(t, "22222222-2222-2222-2222-222222222222"),
		BlobPath:       "sessions/s1/messages/m1/attachments/01-file.png",
		MediaType:      "image/png",
		Filename:       "file.png",
		SizeBytes:      1234,
		Sha256:         "abc",
		ProviderFileID: textValue("file-123"),
		CreatedAt:      mustTime(now),
	}

	attachment, err := mapAttachment(row)
	if err != nil {
		t.Fatalf("mapAttachment() error = %v", err)
	}

	if attachment.ID != "33333333-3333-3333-3333-333333333333" {
		t.Fatalf("attachment.ID = %q", attachment.ID)
	}
	if attachment.ProviderFileID != "file-123" {
		t.Fatalf("attachment.ProviderFileID = %q", attachment.ProviderFileID)
	}
}

func TestMapRun(t *testing.T) {
	startedAt := time.Date(2026, 3, 31, 10, 15, 0, 0, time.UTC)
	completedAt := startedAt.Add(5 * time.Second)
	row := dbsqlc.ChatRun{
		ID:                 mustUUID(t, "44444444-4444-4444-4444-444444444444"),
		SessionID:          mustUUID(t, "11111111-1111-1111-1111-111111111111"),
		UserMessageID:      mustUUID(t, "22222222-2222-2222-2222-222222222222"),
		AssistantMessageID: mustUUID(t, "55555555-5555-5555-5555-555555555555"),
		ProviderResponseID: textValue("resp_123"),
		InputTokens:        mustInt8(45),
		OutputTokens:       mustInt8(78),
		TotalTokens:        mustInt8(123),
		Status:             "completed",
		ErrorCode:          textValue(""),
		StartedAt:          mustTime(startedAt),
		CompletedAt:        mustTime(completedAt),
	}

	run, err := mapRun(row)
	if err != nil {
		t.Fatalf("mapRun() error = %v", err)
	}

	if run.ID != "44444444-4444-4444-4444-444444444444" {
		t.Fatalf("run.ID = %q", run.ID)
	}
	if run.AssistantMessageID != "55555555-5555-5555-5555-555555555555" {
		t.Fatalf("run.AssistantMessageID = %q", run.AssistantMessageID)
	}
	if run.ProviderResponseID != "resp_123" {
		t.Fatalf("run.ProviderResponseID = %q", run.ProviderResponseID)
	}
	if run.InputTokens == nil || *run.InputTokens != 45 {
		t.Fatalf("run.InputTokens = %v", run.InputTokens)
	}
	if run.OutputTokens == nil || *run.OutputTokens != 78 {
		t.Fatalf("run.OutputTokens = %v", run.OutputTokens)
	}
	if run.TotalTokens == nil || *run.TotalTokens != 123 {
		t.Fatalf("run.TotalTokens = %v", run.TotalTokens)
	}
	if run.CompletedAt == nil || !run.CompletedAt.Equal(completedAt) {
		t.Fatalf("run.CompletedAt = %v", run.CompletedAt)
	}
}

func TestParseOptionalUUIDAllowsEmpty(t *testing.T) {
	value, err := parseOptionalUUID("")
	if err != nil {
		t.Fatalf("parseOptionalUUID() error = %v", err)
	}
	if value.Valid {
		t.Fatalf("parseOptionalUUID(\"\") returned valid UUID")
	}
}

func mustUUID(t *testing.T, value string) pgtype.UUID {
	t.Helper()

	var uuid pgtype.UUID
	if err := uuid.Scan(value); err != nil {
		t.Fatalf("Scan(%q): %v", value, err)
	}

	return uuid
}

func mustTime(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{
		Time:  value,
		Valid: true,
	}
}

func mustInt8(value int64) pgtype.Int8 {
	return pgtype.Int8{
		Int64: value,
		Valid: true,
	}
}
