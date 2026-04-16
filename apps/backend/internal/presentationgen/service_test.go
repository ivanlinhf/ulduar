package presentationgen

import (
	"context"
	"encoding/base64"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/ivanlin/ulduar/apps/backend/internal/azureopenai"
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
		generationRead: &stubGenerationReader{
			generation: repository.PresentationGeneration{
				ID:            "22222222-2222-2222-2222-222222222222",
				SessionID:     "11111111-1111-1111-1111-111111111111",
				Prompt:        "build a quarterly review deck",
				DialectJSON:   []byte(`{"version":"v1","slideSize":"16:9","slides":[{"layout":"title","title":"Quarterly review"}]}`),
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
	if string(view.Generation.DialectJSON) != `{"version":"v1","slideSize":"16:9","slides":[{"layout":"title","title":"Quarterly review"}]}` {
		t.Fatalf("view.Generation.DialectJSON = %s", string(view.Generation.DialectJSON))
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
		generationRead: &stubGenerationReader{err: repository.ErrNotFound},
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
	if validationErr.StatusCode != http.StatusNotFound {
		t.Fatalf("validationErr.StatusCode = %d, want %d", validationErr.StatusCode, http.StatusNotFound)
	}
}

func TestGetGenerationReturnsValidationErrorForEmptySessionID(t *testing.T) {
	t.Parallel()

	service := &Service{}

	_, err := service.GetGeneration(context.Background(), "   ", "22222222-2222-2222-2222-222222222222")
	if err == nil {
		t.Fatal("GetGeneration() error = nil, want error")
	}

	validationErr, ok := err.(ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if validationErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("validationErr.StatusCode = %d, want %d", validationErr.StatusCode, http.StatusBadRequest)
	}
	if validationErr.Message != "sessionId must be a valid UUID" {
		t.Fatalf("validationErr.Message = %q, want %q", validationErr.Message, "sessionId must be a valid UUID")
	}
}

func TestExecuteGenerationBuildsAttachmentAwarePlannerRequestAndPersistsNormalizedJSON(t *testing.T) {
	t.Parallel()

	reader := &stubGenerationReader{
		generation: repository.PresentationGeneration{
			ID:        "22222222-2222-2222-2222-222222222222",
			SessionID: "11111111-1111-1111-1111-111111111111",
			Prompt:    "Build a quarterly review deck",
			Status:    string(StatusPending),
		},
		claimPendingResult: true,
	}
	assets := []repository.PresentationGenerationAsset{
		{
			ID:           "asset-image",
			GenerationID: "22222222-2222-2222-2222-222222222222",
			Role:         string(AssetRoleInput),
			SortOrder:    0,
			BlobPath:     "blob://image",
			MediaType:    InputMediaTypePNG,
			Filename:     "reference.png",
			SizeBytes:    3,
		},
		{
			ID:           "asset-pdf",
			GenerationID: "22222222-2222-2222-2222-222222222222",
			Role:         string(AssetRoleInput),
			SortOrder:    1,
			BlobPath:     "blob://pdf",
			MediaType:    InputMediaTypePDF,
			Filename:     "notes.pdf",
			SizeBytes:    4,
		},
	}
	client := &stubResponseClient{
		responses: []azureopenai.Response{{
			Model: "gpt-5-presentation",
			OutputText: `{
				"version": "v1",
				"slides": [
					{"layout": "title", "title": " Quarterly review ", "subtitle": " FY2026 Q1 "}
				]
			}`,
		}},
	}

	service := &Service{
		planner: PlannerConfig{
			Deployment:   "presentation-deployment",
			SystemPrompt: "presentation-system-prompt",
		},
		blobs:          stubBlobStore{data: map[string][]byte{"blob://image": {1, 2, 3}, "blob://pdf": {4, 5, 6, 7}}},
		responses:      client,
		generationRead: reader,
		assetRead:      stubAssetReader{assets: assets},
	}

	if err := service.ExecuteGeneration(context.Background(), "22222222-2222-2222-2222-222222222222"); err != nil {
		t.Fatalf("ExecuteGeneration() error = %v", err)
	}

	if len(client.requests) != 1 {
		t.Fatalf("len(client.requests) = %d, want 1", len(client.requests))
	}
	requestInput, ok := client.requests[0].Input.([]azureopenai.InputMessage)
	if !ok {
		t.Fatalf("request input type = %T, want []azureopenai.InputMessage", client.requests[0].Input)
	}
	if len(requestInput) != 1 {
		t.Fatalf("len(requestInput) = %d, want 1", len(requestInput))
	}
	if got := client.requests[0].Instructions; !strings.Contains(got, "presentation-system-prompt") || !strings.Contains(got, "Return exactly one JSON object and nothing else.") {
		t.Fatalf("request instructions = %q, want system prompt plus planner dialect instructions", got)
	}

	content := requestInput[0].Content
	if len(content) != 3 {
		t.Fatalf("len(content) = %d, want 3", len(content))
	}
	if content[0].Type != providerInputTextType || !strings.Contains(content[0].Text, "Build a quarterly review deck") {
		t.Fatalf("content[0] = %#v, want prompt text item", content[0])
	}
	if content[1].Type != providerInputImageType || !strings.HasPrefix(content[1].ImageURL, "data:image/png;base64,") {
		t.Fatalf("content[1] = %#v, want input image data URL", content[1])
	}
	if content[2].Type != providerInputFileType || content[2].Filename != "notes.pdf" {
		t.Fatalf("content[2] = %#v, want input file attachment", content[2])
	}
	if content[2].FileData != base64.StdEncoding.EncodeToString([]byte{4, 5, 6, 7}) {
		t.Fatalf("content[2].FileData = %q, want encoded PDF bytes", content[2].FileData)
	}

	if len(reader.updateCalls) != 1 {
		t.Fatalf("len(reader.updateCalls) = %d, want 1", len(reader.updateCalls))
	}
	update := reader.updateCalls[0]
	if update.Status != string(StatusCompleted) {
		t.Fatalf("update.Status = %q, want %q", update.Status, StatusCompleted)
	}
	if update.ProviderName != plannerProviderName {
		t.Fatalf("update.ProviderName = %q, want %q", update.ProviderName, plannerProviderName)
	}
	if update.ProviderModel != "gpt-5-presentation" {
		t.Fatalf("update.ProviderModel = %q, want %q", update.ProviderModel, "gpt-5-presentation")
	}
	if got := string(update.DialectJSON); got != `{"version":"v1","slideSize":"16:9","slides":[{"layout":"title","title":"Quarterly review","subtitle":"FY2026 Q1"}]}` {
		t.Fatalf("update.DialectJSON = %s", got)
	}
}

func TestExecuteGenerationRetriesRepairOnInvalidPlannerOutput(t *testing.T) {
	t.Parallel()

	reader := &stubGenerationReader{
		generation: repository.PresentationGeneration{
			ID:        "22222222-2222-2222-2222-222222222222",
			SessionID: "11111111-1111-1111-1111-111111111111",
			Prompt:    "Build a quarterly review deck",
			Status:    string(StatusPending),
		},
		claimPendingResult: true,
	}
	client := &stubResponseClient{
		responses: []azureopenai.Response{
			{OutputText: `{"version":"v1","slides":[{"layout":"title"}]}`},
			{OutputText: `{"version":"v1","slides":[{"layout":"title","title":"Fixed title"}]}`},
		},
	}

	service := &Service{
		planner:        PlannerConfig{Deployment: "presentation-deployment"},
		responses:      client,
		generationRead: reader,
		assetRead:      stubAssetReader{},
	}

	if err := service.ExecuteGeneration(context.Background(), "22222222-2222-2222-2222-222222222222"); err != nil {
		t.Fatalf("ExecuteGeneration() error = %v", err)
	}

	if len(client.requests) != 2 {
		t.Fatalf("len(client.requests) = %d, want 2", len(client.requests))
	}
	repairInput, ok := client.requests[1].Input.([]azureopenai.InputMessage)
	if !ok {
		t.Fatalf("repair request input type = %T, want []azureopenai.InputMessage", client.requests[1].Input)
	}
	if len(repairInput) != 2 {
		t.Fatalf("len(repairInput) = %d, want 2", len(repairInput))
	}
	if !strings.Contains(repairInput[1].Content[0].Text, `slides[0].title is required`) {
		t.Fatalf("repair prompt = %q, want validation error details", repairInput[1].Content[0].Text)
	}
	if len(reader.updateCalls) != 1 || reader.updateCalls[0].Status != string(StatusCompleted) {
		t.Fatalf("updateCalls = %#v, want completed update after repair", reader.updateCalls)
	}
}

func TestExecuteGenerationLeavesFreshRunningGenerationUntouched(t *testing.T) {
	t.Parallel()

	startedAt := time.Now().UTC().Add(-90 * time.Second)
	reader := &stubGenerationReader{
		generation: repository.PresentationGeneration{
			ID:        "22222222-2222-2222-2222-222222222222",
			SessionID: "11111111-1111-1111-1111-111111111111",
			Status:    string(StatusRunning),
			StartedAt: &startedAt,
		},
	}

	service := &Service{
		planner:        PlannerConfig{RequestTimeout: 2 * time.Minute},
		responses:      &stubResponseClient{},
		generationRead: reader,
		assetRead:      stubAssetReader{},
	}

	if err := service.ExecuteGeneration(context.Background(), "22222222-2222-2222-2222-222222222222"); err != nil {
		t.Fatalf("ExecuteGeneration() error = %v", err)
	}
	if len(reader.updateCalls) != 0 {
		t.Fatalf("len(reader.updateCalls) = %d, want 0", len(reader.updateCalls))
	}
}

func TestExecuteGenerationFailsStaleRunningGeneration(t *testing.T) {
	t.Parallel()

	startedAt := time.Now().UTC().Add(-(90*time.Second + runningGracePeriod + time.Second))
	reader := &stubGenerationReader{
		generation: repository.PresentationGeneration{
			ID:            "22222222-2222-2222-2222-222222222222",
			SessionID:     "11111111-1111-1111-1111-111111111111",
			ProviderName:  plannerProviderName,
			ProviderModel: "presentation-deployment",
			Status:        string(StatusRunning),
			StartedAt:     &startedAt,
		},
	}

	service := &Service{
		planner:        PlannerConfig{RequestTimeout: 90 * time.Second},
		responses:      &stubResponseClient{},
		generationRead: reader,
		assetRead:      stubAssetReader{},
	}

	if err := service.ExecuteGeneration(context.Background(), "22222222-2222-2222-2222-222222222222"); err != nil {
		t.Fatalf("ExecuteGeneration() error = %v", err)
	}
	if len(reader.updateCalls) != 1 {
		t.Fatalf("len(reader.updateCalls) = %d, want 1", len(reader.updateCalls))
	}
	update := reader.updateCalls[0]
	if update.Status != string(StatusFailed) {
		t.Fatalf("update.Status = %q, want %q", update.Status, StatusFailed)
	}
	if update.ErrorCode != "planner_stale_running" {
		t.Fatalf("update.ErrorCode = %q, want %q", update.ErrorCode, "planner_stale_running")
	}
}

func TestExecuteGenerationFailsWhenPlannerOutputRemainsInvalidAfterRepair(t *testing.T) {
	t.Parallel()

	reader := &stubGenerationReader{
		generation: repository.PresentationGeneration{
			ID:        "22222222-2222-2222-2222-222222222222",
			SessionID: "11111111-1111-1111-1111-111111111111",
			Prompt:    "Build a quarterly review deck",
			Status:    string(StatusPending),
		},
		claimPendingResult: true,
	}
	client := &stubResponseClient{
		responses: []azureopenai.Response{
			{OutputText: `not json`},
			{OutputText: `still not json`},
		},
	}

	service := &Service{
		planner:        PlannerConfig{Deployment: "presentation-deployment"},
		responses:      client,
		generationRead: reader,
		assetRead:      stubAssetReader{},
	}

	err := service.ExecuteGeneration(context.Background(), "22222222-2222-2222-2222-222222222222")
	if err == nil {
		t.Fatal("ExecuteGeneration() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "plan presentation") {
		t.Fatalf("ExecuteGeneration() error = %q, want planner failure", err.Error())
	}
	if len(reader.updateCalls) != 1 {
		t.Fatalf("len(reader.updateCalls) = %d, want 1", len(reader.updateCalls))
	}
	update := reader.updateCalls[0]
	if update.Status != string(StatusFailed) {
		t.Fatalf("update.Status = %q, want %q", update.Status, StatusFailed)
	}
	if update.ErrorCode != plannerFailureInvalidJSON {
		t.Fatalf("update.ErrorCode = %q, want %q", update.ErrorCode, plannerFailureInvalidJSON)
	}
	if !strings.Contains(update.ErrorMessage, "decode presentation document") {
		t.Fatalf("update.ErrorMessage = %q, want decode failure", update.ErrorMessage)
	}
}

func TestPrepareAttachmentInputRejectsOversizedAsset(t *testing.T) {
	t.Parallel()

	service := &Service{
		blobs: stubBlobStore{
			data: map[string][]byte{
				"blob://large": []byte("ignored"),
			},
		},
	}

	_, err := service.prepareAttachmentInput(context.Background(), repository.PresentationGenerationAsset{
		ID:        "asset-large",
		Role:      string(AssetRoleInput),
		BlobPath:  "blob://large",
		MediaType: InputMediaTypePDF,
		Filename:  "large.pdf",
		SizeBytes: defaultMaxAttachmentBytes + 1,
	})
	if err == nil {
		t.Fatal("prepareAttachmentInput() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("prepareAttachmentInput() error = %q, want size limit error", err.Error())
	}
}

type stubGenerationReader struct {
	generation          repository.PresentationGeneration
	err                 error
	claimPendingResult  bool
	claimPendingErr     error
	updateCalls         []repository.UpdatePresentationGenerationStateParams
}

func (s *stubGenerationReader) GetByID(context.Context, string) (repository.PresentationGeneration, error) {
	if s.err != nil {
		return repository.PresentationGeneration{}, s.err
	}

	return s.generation, nil
}

func (s *stubGenerationReader) GetByIDAndSession(context.Context, string, string) (repository.PresentationGeneration, error) {
	if s.err != nil {
		return repository.PresentationGeneration{}, s.err
	}

	return s.generation, nil
}

func (s *stubGenerationReader) ClaimPending(context.Context, repository.ClaimPendingPresentationGenerationParams) (bool, error) {
	if s.claimPendingErr != nil {
		return false, s.claimPendingErr
	}

	return s.claimPendingResult, nil
}

func (s *stubGenerationReader) UpdateState(_ context.Context, params repository.UpdatePresentationGenerationStateParams) error {
	s.updateCalls = append(s.updateCalls, params)
	return nil
}

type stubAssetReader struct {
	assets []repository.PresentationGenerationAsset
	err    error
}

func (s stubAssetReader) ListByGeneration(context.Context, string) ([]repository.PresentationGenerationAsset, error) {
	if s.err != nil {
		return nil, s.err
	}

	return s.assets, nil
}

func (s stubAssetReader) ListByGenerationAndSession(context.Context, string, string) ([]repository.PresentationGenerationAsset, error) {
	if s.err != nil {
		return nil, s.err
	}

	return s.assets, nil
}

type stubResponseClient struct {
	responses []azureopenai.Response
	err       error
	requests  []azureopenai.CreateResponseRequest
}

func (s *stubResponseClient) CreateResponse(_ context.Context, request azureopenai.CreateResponseRequest) (azureopenai.Response, error) {
	s.requests = append(s.requests, request)
	if s.err != nil {
		return azureopenai.Response{}, s.err
	}
	if len(s.responses) == 0 {
		return azureopenai.Response{}, nil
	}

	response := s.responses[0]
	s.responses = s.responses[1:]
	return response, nil
}

type stubBlobStore struct {
	data map[string][]byte
}

func (s stubBlobStore) Download(_ context.Context, blobPath string) ([]byte, error) {
	data, ok := s.data[blobPath]
	if !ok {
		return nil, context.Canceled
	}

	return data, nil
}

func (s stubBlobStore) DownloadWithinLimit(ctx context.Context, blobPath string, maxBytes int64) ([]byte, error) {
	data, err := s.Download(ctx, blobPath)
	if err != nil {
		return nil, err
	}
	if maxBytes > 0 && int64(len(data)) > maxBytes {
		return nil, context.DeadlineExceeded
	}

	return data, nil
}
