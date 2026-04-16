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

func TestMapImageGeneration(t *testing.T) {
	createdAt := time.Date(2026, 3, 31, 10, 20, 0, 0, time.UTC)
	startedAt := createdAt.Add(time.Minute)
	completedAt := createdAt.Add(3 * time.Minute)
	row := dbsqlc.ImageGeneration{
		ID:                  mustUUID(t, "66666666-6666-6666-6666-666666666666"),
		SessionID:           mustUUID(t, "11111111-1111-1111-1111-111111111111"),
		Mode:                "text_to_image",
		Prompt:              "a castle at sunset",
		ResolutionKey:       "1024x1024",
		Width:               1024,
		Height:              1024,
		RequestedImageCount: 1,
		ProviderName:        "example-provider",
		ProviderModel:       "example-model",
		ProviderJobID:       textValue("job-123"),
		Status:              "completed",
		ErrorCode:           textValue(""),
		ErrorMessage:        textValue(""),
		CreatedAt:           mustTime(createdAt),
		StartedAt:           mustTime(startedAt),
		CompletedAt:         mustTime(completedAt),
	}

	generation, err := mapImageGeneration(row)
	if err != nil {
		t.Fatalf("mapImageGeneration() error = %v", err)
	}

	if generation.ID != "66666666-6666-6666-6666-666666666666" {
		t.Fatalf("generation.ID = %q", generation.ID)
	}
	if generation.RequestedImageCount != 1 {
		t.Fatalf("generation.RequestedImageCount = %d", generation.RequestedImageCount)
	}
	if generation.ProviderJobID != "job-123" {
		t.Fatalf("generation.ProviderJobID = %q", generation.ProviderJobID)
	}
	if generation.StartedAt == nil || !generation.StartedAt.Equal(startedAt) {
		t.Fatalf("generation.StartedAt = %v", generation.StartedAt)
	}
	if generation.CompletedAt == nil || !generation.CompletedAt.Equal(completedAt) {
		t.Fatalf("generation.CompletedAt = %v", generation.CompletedAt)
	}
}

func TestMapImageGenerationAsset(t *testing.T) {
	createdAt := time.Date(2026, 3, 31, 10, 25, 0, 0, time.UTC)
	row := dbsqlc.ImageGenerationAsset{
		ID:           mustUUID(t, "77777777-7777-7777-7777-777777777777"),
		GenerationID: mustUUID(t, "66666666-6666-6666-6666-666666666666"),
		Role:         "output",
		SortOrder:    0,
		BlobPath:     "sessions/s1/image-generations/g1/output-0.png",
		MediaType:    "image/png",
		Filename:     "output-0.png",
		SizeBytes:    2048,
		Sha256:       "def",
		Width:        mustInt8(1024),
		Height:       mustInt8(1024),
		CreatedAt:    mustTime(createdAt),
	}

	asset, err := mapImageGenerationAsset(row)
	if err != nil {
		t.Fatalf("mapImageGenerationAsset() error = %v", err)
	}

	if asset.ID != "77777777-7777-7777-7777-777777777777" {
		t.Fatalf("asset.ID = %q", asset.ID)
	}
	if asset.Width == nil || *asset.Width != 1024 {
		t.Fatalf("asset.Width = %v", asset.Width)
	}
	if asset.Height == nil || *asset.Height != 1024 {
		t.Fatalf("asset.Height = %v", asset.Height)
	}
}

func TestMapPresentationGeneration(t *testing.T) {
	createdAt := time.Date(2026, 4, 16, 2, 5, 0, 0, time.UTC)
	startedAt := createdAt.Add(time.Minute)
	completedAt := createdAt.Add(2 * time.Minute)
	row := dbsqlc.PresentationGeneration{
		ID:            mustUUID(t, "88888888-8888-8888-8888-888888888888"),
		SessionID:     mustUUID(t, "11111111-1111-1111-1111-111111111111"),
		Prompt:        "prepare a roadmap deck",
		ProviderName:  "azure-openai",
		ProviderModel: "gpt-5-chat",
		ProviderJobID: textValue("job-456"),
		Status:        "completed",
		ErrorCode:     textValue(""),
		ErrorMessage:  textValue(""),
		CreatedAt:     mustTime(createdAt),
		StartedAt:     mustTime(startedAt),
		CompletedAt:   mustTime(completedAt),
	}

	generation, err := mapPresentationGeneration(row)
	if err != nil {
		t.Fatalf("mapPresentationGeneration() error = %v", err)
	}

	if generation.ID != "88888888-8888-8888-8888-888888888888" {
		t.Fatalf("generation.ID = %q", generation.ID)
	}
	if generation.ProviderJobID != "job-456" {
		t.Fatalf("generation.ProviderJobID = %q", generation.ProviderJobID)
	}
	if generation.StartedAt == nil || !generation.StartedAt.Equal(startedAt) {
		t.Fatalf("generation.StartedAt = %v", generation.StartedAt)
	}
	if generation.CompletedAt == nil || !generation.CompletedAt.Equal(completedAt) {
		t.Fatalf("generation.CompletedAt = %v", generation.CompletedAt)
	}
}

func TestMapPresentationGenerationAsset(t *testing.T) {
	createdAt := time.Date(2026, 4, 16, 2, 10, 0, 0, time.UTC)
	row := dbsqlc.PresentationGenerationAsset{
		ID:           mustUUID(t, "99999999-9999-9999-9999-999999999999"),
		GenerationID: mustUUID(t, "88888888-8888-8888-8888-888888888888"),
		Role:         "output",
		SortOrder:    0,
		BlobPath:     "sessions/s1/presentation-generations/g1/outputs/final.pptx",
		MediaType:    "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		Filename:     "final.pptx",
		SizeBytes:    8192,
		Sha256:       "ghi",
		CreatedAt:    mustTime(createdAt),
	}

	asset, err := mapPresentationGenerationAsset(row)
	if err != nil {
		t.Fatalf("mapPresentationGenerationAsset() error = %v", err)
	}

	if asset.ID != "99999999-9999-9999-9999-999999999999" {
		t.Fatalf("asset.ID = %q", asset.ID)
	}
	if asset.MediaType != "application/vnd.openxmlformats-officedocument.presentationml.presentation" {
		t.Fatalf("asset.MediaType = %q", asset.MediaType)
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
