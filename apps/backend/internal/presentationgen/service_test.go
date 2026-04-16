package presentationgen

import (
	"context"
	"testing"
	"time"

	"github.com/ivanlin/ulduar/apps/backend/internal/repository"
)

func TestSupportedInputMediaTypesReturnsCopy(t *testing.T) {
	t.Parallel()

	mediaTypes := SupportedInputMediaTypes()
	if len(mediaTypes) != 4 {
		t.Fatalf("len(mediaTypes) = %d, want 4", len(mediaTypes))
	}

	want := []string{
		InputMediaTypeJPEG,
		InputMediaTypePNG,
		InputMediaTypeWEBP,
		InputMediaTypePDF,
	}
	for index, value := range want {
		if mediaTypes[index] != value {
			t.Fatalf("mediaTypes[%d] = %q, want %q", index, mediaTypes[index], value)
		}
	}

	mediaTypes[0] = "changed"
	if SupportedInputMediaTypes()[0] != InputMediaTypeJPEG {
		t.Fatal("SupportedInputMediaTypes() did not return a copy")
	}
}

func TestNewServicePanicsOnMultipleOptions(t *testing.T) {
	t.Parallel()

	defer func() {
		if recovered := recover(); recovered == nil {
			t.Fatal("expected panic, got nil")
		}
	}()

	_ = NewService(nil, ServiceOptions{}, ServiceOptions{})
}

func TestPlannerConfigured(t *testing.T) {
	t.Parallel()

	if NewService(nil).PlannerConfigured() {
		t.Fatal("PlannerConfigured() = true, want false")
	}

	service := NewService(nil, ServiceOptions{
		Planner: PlannerConfig{
			Endpoint: "https://example.openai.azure.com/",
		},
	})
	if !service.PlannerConfigured() {
		t.Fatal("PlannerConfigured() = false, want true")
	}
}

func TestGetGeneration(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, 4, 16, 2, 0, 0, 0, time.UTC)
	startedAt := createdAt.Add(time.Minute)
	completedAt := createdAt.Add(2 * time.Minute)

	service := &Service{
		generationRead: stubGenerationReader{
			generation: repository.PresentationGeneration{
				ID:            "22222222-2222-2222-2222-222222222222",
				SessionID:     "11111111-1111-1111-1111-111111111111",
				Prompt:        "build a quarterly review deck",
				ProviderName:  "azure-openai",
				ProviderModel: "gpt-5-chat",
				ProviderJobID: "job-123",
				Status:        string(StatusCompleted),
				CreatedAt:     createdAt,
				StartedAt:     &startedAt,
				CompletedAt:   &completedAt,
			},
		},
		assetRead: stubAssetReader{
			assets: []repository.PresentationGenerationAsset{
				{
					ID:           "33333333-3333-3333-3333-333333333333",
					GenerationID: "22222222-2222-2222-2222-222222222222",
					Role:         string(AssetRoleOutput),
					SortOrder:    0,
					BlobPath:     "sessions/s1/presentation-generations/g1/outputs/final.pptx",
					MediaType:    OutputMediaTypePPTX,
					Filename:     "final.pptx",
					SizeBytes:    4096,
					Sha256:       "abc123",
					CreatedAt:    completedAt,
				},
			},
		},
	}

	view, err := service.GetGeneration(context.Background(), "11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222")
	if err != nil {
		t.Fatalf("GetGeneration() error = %v", err)
	}

	if view.Generation.Status != StatusCompleted {
		t.Fatalf("view.Generation.Status = %q", view.Generation.Status)
	}
	if len(view.Assets) != 1 {
		t.Fatalf("len(view.Assets) = %d, want 1", len(view.Assets))
	}
	if view.Assets[0].MediaType != OutputMediaTypePPTX {
		t.Fatalf("view.Assets[0].MediaType = %q", view.Assets[0].MediaType)
	}
}

func TestGetGenerationMapsRepositoryNotFound(t *testing.T) {
	t.Parallel()

	service := &Service{
		generationRead: stubGenerationReader{err: repository.ErrNotFound},
		assetRead:      stubAssetReader{},
	}

	_, err := service.GetGeneration(context.Background(), "11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222")
	if err == nil {
		t.Fatal("GetGeneration() error = nil, want error")
	}

	validationErr, ok := err.(ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if validationErr.StatusCode != 404 {
		t.Fatalf("validationErr.StatusCode = %d, want 404", validationErr.StatusCode)
	}
}

type stubGenerationReader struct {
	generation repository.PresentationGeneration
	err        error
}

func (s stubGenerationReader) GetByIDAndSession(context.Context, string, string) (repository.PresentationGeneration, error) {
	if s.err != nil {
		return repository.PresentationGeneration{}, s.err
	}

	return s.generation, nil
}

type stubAssetReader struct {
	assets []repository.PresentationGenerationAsset
	err    error
}

func (s stubAssetReader) ListByGenerationAndSession(context.Context, string, string) ([]repository.PresentationGenerationAsset, error) {
	if s.err != nil {
		return nil, s.err
	}

	return s.assets, nil
}
