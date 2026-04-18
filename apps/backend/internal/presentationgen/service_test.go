package presentationgen

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/ivanlin/ulduar/apps/backend/internal/azureopenai"
	"github.com/ivanlin/ulduar/apps/backend/internal/blobstorage"
	"github.com/ivanlin/ulduar/apps/backend/internal/presentationdialect"
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
	if service.PlannerConfigured() {
		t.Fatal("PlannerConfigured() = true, want false when response client is missing")
	}

	service = NewService(nil, ServiceOptions{
		Planner: PlannerConfig{
			Endpoint: "https://example.openai.azure.com/",
		},
		ResponseClient: &stubResponseClient{},
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
				ID:                "22222222-2222-2222-2222-222222222222",
				SessionID:         "11111111-1111-1111-1111-111111111111",
				Prompt:            "build a quarterly review deck",
				PlannerOutputJSON: []byte(`{"version":"v2","themePresetId":"travel_editorial","slides":[{"layout":"cover_hero","title":"Quarterly review","blocks":[{"type":"image","assetRef":"attachment:reference"}]}]}`),
				DialectJSON:       []byte(`{"version":"v1","slideSize":"16:9","slides":[{"layout":"title","title":"Quarterly review"}]}`),
				ProviderName:      "azure-openai",
				ProviderModel:     "gpt-5-chat",
				ProviderJobID:     "job-123",
				Status:            string(StatusCompleted),
				CreatedAt:         createdAt,
				StartedAt:         &startedAt,
				CompletedAt:       &completedAt,
			},
		},
		assetRead: stubAssetReader{
			assets: []repository.PresentationGenerationAsset{
				{
					ID:            "44444444-4444-4444-4444-444444444444",
					GenerationID:  "22222222-2222-2222-2222-222222222222",
					Role:          string(AssetRoleResolved),
					AssetRef:      "attachment:reference",
					SourceType:    string(AssetSourceTypeInputAsset),
					SourceAssetID: "33333333-3333-3333-3333-333333333333",
					BlobPath:      "sessions/s1/presentation-generations/g1/inputs/01-reference.png",
					MediaType:     InputMediaTypePNG,
					Filename:      "reference.png",
					SizeBytes:     1024,
					Sha256:        "resolved-sha",
					CreatedAt:     completedAt,
				},
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
	if string(view.Generation.PlannerOutputJSON) != `{"version":"v2","themePresetId":"travel_editorial","slides":[{"layout":"cover_hero","title":"Quarterly review","blocks":[{"type":"image","assetRef":"attachment:reference"}]}]}` {
		t.Fatalf("view.Generation.PlannerOutputJSON = %s", string(view.Generation.PlannerOutputJSON))
	}
	if len(view.Assets) != 2 {
		t.Fatalf("len(view.Assets) = %d, want 2", len(view.Assets))
	}
	if view.Assets[0].Role != AssetRoleResolved || view.Assets[0].AssetRef != "attachment:reference" {
		t.Fatalf("view.Assets[0] = %#v", view.Assets[0])
	}
	if view.Assets[1].MediaType != OutputMediaTypePPTX {
		t.Fatalf("view.Assets[1].MediaType = %q", view.Assets[1].MediaType)
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

func TestCreatePendingGenerationStoresPromptAndInputAssets(t *testing.T) {
	t.Parallel()

	tx := &stubWriteTx{
		session: repository.Session{ID: "11111111-1111-1111-1111-111111111111"},
		generation: repository.PresentationGeneration{
			ID:        "22222222-2222-2222-2222-222222222222",
			SessionID: "11111111-1111-1111-1111-111111111111",
			Status:    string(StatusPending),
			CreatedAt: time.Date(2026, 4, 16, 9, 0, 0, 0, time.UTC),
		},
	}
	blobs := &stubBlobStore{}
	service := &Service{
		blobs:          blobs,
		beginWriteTxFn: func(context.Context) (writeTx, error) { return tx, nil },
	}

	view, err := service.CreatePendingGeneration(context.Background(), CreateGenerationParams{
		SessionID: "11111111-1111-1111-1111-111111111111",
		Prompt:    " Build a quarterly review deck ",
		Attachments: []InputAssetUpload{
			{Filename: "reference.png", Data: testPNGData()},
			{Filename: "../notes.pdf", Data: []byte("%PDF-1.7\n1 0 obj\n<<>>\nendobj\n")},
		},
	})
	if err != nil {
		t.Fatalf("CreatePendingGeneration() error = %v", err)
	}

	if tx.createGenerationParams.Prompt != "Build a quarterly review deck" {
		t.Fatalf("tx.createGenerationParams.Prompt = %q", tx.createGenerationParams.Prompt)
	}
	if len(tx.createAssetCalls) != 2 {
		t.Fatalf("len(tx.createAssetCalls) = %d, want 2", len(tx.createAssetCalls))
	}
	if !tx.committed {
		t.Fatal("transaction was not committed")
	}
	if len(blobs.uploadCalls) != 2 {
		t.Fatalf("len(blobs.uploadCalls) = %d, want 2", len(blobs.uploadCalls))
	}
	if view.Generation.ID != "22222222-2222-2222-2222-222222222222" {
		t.Fatalf("view.Generation.ID = %q", view.Generation.ID)
	}
	if len(view.Assets) != 2 {
		t.Fatalf("len(view.Assets) = %d, want 2", len(view.Assets))
	}
}

func TestGetAssetContentReturnsOutputAsset(t *testing.T) {
	t.Parallel()

	service := &Service{
		blobs: &stubBlobStore{
			data: map[string][]byte{
				"blob://output": []byte("pptx-data"),
			},
		},
		assetRead: stubAssetReader{
			assetByID: repository.PresentationGenerationAsset{
				ID:           "33333333-3333-3333-3333-333333333333",
				GenerationID: "22222222-2222-2222-2222-222222222222",
				Role:         string(AssetRoleOutput),
				BlobPath:     "blob://output",
				MediaType:    OutputMediaTypePPTX,
				Filename:     "final.pptx",
				SizeBytes:    int64(len("pptx-data")),
			},
		},
	}

	content, err := service.GetAssetContent(context.Background(), "11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222", "33333333-3333-3333-3333-333333333333")
	if err != nil {
		t.Fatalf("GetAssetContent() error = %v", err)
	}
	if content.MediaType != OutputMediaTypePPTX {
		t.Fatalf("content.MediaType = %q", content.MediaType)
	}
	if string(content.Data) != "pptx-data" {
		t.Fatalf("content.Data = %q", string(content.Data))
	}
}

func TestGetAssetContentRejectsInvalidStoredSize(t *testing.T) {
	t.Parallel()

	service := &Service{
		blobs: &stubBlobStore{
			data: map[string][]byte{
				"blob://output": []byte("pptx-data"),
			},
		},
		assetRead: stubAssetReader{
			assetByID: repository.PresentationGenerationAsset{
				ID:           "33333333-3333-3333-3333-333333333333",
				GenerationID: "22222222-2222-2222-2222-222222222222",
				Role:         string(AssetRoleOutput),
				BlobPath:     "blob://output",
				MediaType:    OutputMediaTypePPTX,
				Filename:     "final.pptx",
				SizeBytes:    0,
			},
		},
	}

	_, err := service.GetAssetContent(context.Background(), "11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222", "33333333-3333-3333-3333-333333333333")
	if err == nil {
		t.Fatal("GetAssetContent() error = nil, want invalid size error")
	}
	if !strings.Contains(err.Error(), "presentation generation asset size is invalid") {
		t.Fatalf("GetAssetContent() error = %q", err.Error())
	}
}

func TestCompleteGenerationSkipsCompileWhenGenerationAlreadyTerminal(t *testing.T) {
	t.Parallel()

	completedAt := time.Date(2026, 4, 17, 6, 0, 0, 0, time.UTC)
	tx := &stubWriteTx{
		lockedGeneration: repository.PresentationGeneration{
			ID:          "22222222-2222-2222-2222-222222222222",
			SessionID:   "11111111-1111-1111-1111-111111111111",
			Status:      string(StatusCompleted),
			CompletedAt: &completedAt,
		},
	}
	blobs := &stubBlobStore{}
	service := &Service{
		blobs:          blobs,
		beginWriteTxFn: func(context.Context) (writeTx, error) { return tx, nil },
		generationRead: &stubGenerationReader{},
	}

	err := service.completeGeneration(context.Background(), repository.PresentationGeneration{
		ID:            "22222222-2222-2222-2222-222222222222",
		SessionID:     "11111111-1111-1111-1111-111111111111",
		ProviderName:  plannerProviderName,
		ProviderModel: "presentation-deployment",
		Status:        string(StatusRunning),
	}, nil, azureopenai.Response{}, nil, []byte(`{"version":"v1","slideSize":"16:9","slides":[{"layout":"title","title":"Quarterly review"}]}`))
	if err != nil {
		t.Fatalf("completeGeneration() error = %v", err)
	}
	if len(tx.createAssetCalls) != 0 {
		t.Fatalf("len(tx.createAssetCalls) = %d, want 0", len(tx.createAssetCalls))
	}
	if len(tx.updateCalls) != 0 {
		t.Fatalf("len(tx.updateCalls) = %d, want 0", len(tx.updateCalls))
	}
	if len(blobs.uploadCalls) != 1 {
		t.Fatalf("len(blobs.uploadCalls) = %d, want 1", len(blobs.uploadCalls))
	}
	if len(blobs.deleteCalls) != 1 {
		t.Fatalf("len(blobs.deleteCalls) = %d, want 1", len(blobs.deleteCalls))
	}
	if !tx.rolledBack {
		t.Fatal("expected transaction rollback when generation is already terminal")
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
			Filename:     "../unsafe/notes.pdf",
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
	tx := &stubWriteTx{
		lockedGeneration: repository.PresentationGeneration{
			ID:            "22222222-2222-2222-2222-222222222222",
			SessionID:     "11111111-1111-1111-1111-111111111111",
			ProviderName:  plannerProviderName,
			ProviderModel: "presentation-deployment",
			Status:        string(StatusRunning),
		},
	}

	service := &Service{
		planner: PlannerConfig{
			Deployment:   "presentation-deployment",
			SystemPrompt: "presentation-system-prompt",
		},
		blobs:          &stubBlobStore{data: map[string][]byte{"blob://image": {1, 2, 3}, "blob://pdf": {4, 5, 6, 7}}},
		responses:      client,
		generationRead: reader,
		assetRead:      stubAssetReader{assets: assets},
		beginWriteTxFn: func(context.Context) (writeTx, error) { return tx, nil },
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
	if len(client.requests[0].Tools) != 0 {
		t.Fatalf("len(client.requests[0].Tools) = %d, want 0 by default", len(client.requests[0].Tools))
	}

	content := requestInput[0].Content
	if len(content) != 4 {
		t.Fatalf("len(content) = %d, want 4", len(content))
	}
	if content[0].Type != providerInputTextType || !strings.Contains(content[0].Text, "Build a quarterly review deck") {
		t.Fatalf("content[0] = %#v, want prompt text item", content[0])
	}
	if content[1].Type != providerInputTextType || !strings.Contains(content[1].Text, "attachment:reference") || !strings.Contains(content[1].Text, "attachment:notes") {
		t.Fatalf("content[1] = %#v, want attachment asset guidance", content[1])
	}
	if content[2].Type != providerInputImageType || !strings.HasPrefix(content[2].ImageURL, "data:image/png;base64,") {
		t.Fatalf("content[2] = %#v, want input image data URL", content[2])
	}
	if content[3].Type != providerInputFileType || content[3].Filename != "notes.pdf" {
		t.Fatalf("content[3] = %#v, want input file attachment", content[3])
	}
	if strings.ContainsAny(content[3].Filename, `/\`) {
		t.Fatalf("content[3].Filename = %q, want sanitized basename without path separators", content[3].Filename)
	}
	if content[3].FileData != base64.StdEncoding.EncodeToString([]byte{4, 5, 6, 7}) {
		t.Fatalf("content[3].FileData = %q, want encoded PDF bytes", content[3].FileData)
	}

	if len(tx.updateCalls) != 1 {
		t.Fatalf("len(tx.updateCalls) = %d, want 1", len(tx.updateCalls))
	}
	update := tx.updateCalls[0]
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
	if got := string(update.PlannerOutputJSON); got != `{
				"version": "v1",
				"slides": [
					{"layout": "title", "title": " Quarterly review ", "subtitle": " FY2026 Q1 "}
				]
			}` {
		t.Fatalf("update.PlannerOutputJSON = %s", got)
	}
	if len(tx.createAssetCalls) != 1 {
		t.Fatalf("len(tx.createAssetCalls) = %d, want 1", len(tx.createAssetCalls))
	}
	if tx.createAssetCalls[0].Filename != "final.pptx" {
		t.Fatalf("tx.createAssetCalls[0].Filename = %q", tx.createAssetCalls[0].Filename)
	}
	blobStore := service.blobs.(*stubBlobStore)
	if len(blobStore.uploadCalls) != 1 {
		t.Fatalf("len(blobStore.uploadCalls) = %d, want 1", len(blobStore.uploadCalls))
	}
	if blobStore.uploadCalls[0].contentType != OutputMediaTypePPTX {
		t.Fatalf("blobStore.uploadCalls[0].contentType = %q", blobStore.uploadCalls[0].contentType)
	}
}

func TestExecuteGenerationPersistsResolvedAssetsForAttachmentAndThemeRefs(t *testing.T) {
	t.Parallel()

	reader := &stubGenerationReader{
		generation: repository.PresentationGeneration{
			ID:        "22222222-2222-2222-2222-222222222222",
			SessionID: "11111111-1111-1111-1111-111111111111",
			Prompt:    "Build a Kyoto travel deck",
			Status:    string(StatusPending),
		},
		claimPendingResult: true,
	}
	assets := []repository.PresentationGenerationAsset{{
		ID:           "asset-cover",
		GenerationID: "22222222-2222-2222-2222-222222222222",
		Role:         string(AssetRoleInput),
		SortOrder:    0,
		BlobPath:     "blob://cover",
		MediaType:    InputMediaTypePNG,
		Filename:     "cover.png",
		SizeBytes:    3,
		Sha256:       "cover-sha",
	}}
	client := &stubResponseClient{
		responses: []azureopenai.Response{{
			Model: "gpt-5-presentation",
			OutputText: `{
				"version": "v2",
				"themePresetId": "travel_editorial",
				"slides": [
					{"layout": "cover_hero", "title": "Kyoto", "blocks": [{"type": "image", "assetRef": "attachment:cover"}]},
					{"layout": "chapter_divider", "title": "Highlights", "blocks": [{"type": "image", "assetRef": "theme:hero-image"}]}
				]
			}`,
		}},
	}
	tx := &stubWriteTx{
		lockedGeneration: repository.PresentationGeneration{
			ID:            "22222222-2222-2222-2222-222222222222",
			SessionID:     "11111111-1111-1111-1111-111111111111",
			ProviderName:  plannerProviderName,
			ProviderModel: "presentation-deployment",
			Status:        string(StatusRunning),
		},
	}
	blobs := &stubBlobStore{data: map[string][]byte{"blob://cover": {1, 2, 3}}}
	service := &Service{
		planner:        PlannerConfig{Deployment: "presentation-deployment"},
		blobs:          blobs,
		responses:      client,
		generationRead: reader,
		assetRead:      stubAssetReader{assets: assets},
		beginWriteTxFn: func(context.Context) (writeTx, error) { return tx, nil },
		assetResolver:  newDefaultAssetResolver(blobs),
	}

	if err := service.ExecuteGeneration(context.Background(), "22222222-2222-2222-2222-222222222222"); err != nil {
		t.Fatalf("ExecuteGeneration() error = %v", err)
	}

	if len(tx.createAssetCalls) != 3 {
		t.Fatalf("len(tx.createAssetCalls) = %d, want 3", len(tx.createAssetCalls))
	}
	if tx.createAssetCalls[0].Role != string(AssetRoleResolved) || tx.createAssetCalls[0].AssetRef != "attachment:cover" || tx.createAssetCalls[0].SourceAssetID != "asset-cover" {
		t.Fatalf("tx.createAssetCalls[0] = %#v", tx.createAssetCalls[0])
	}
	if tx.createAssetCalls[1].Role != string(AssetRoleResolved) || tx.createAssetCalls[1].AssetRef != "theme:hero-image" || tx.createAssetCalls[1].SourceType != string(AssetSourceTypeThemeBundle) {
		t.Fatalf("tx.createAssetCalls[1] = %#v", tx.createAssetCalls[1])
	}
	if tx.createAssetCalls[2].Role != string(AssetRoleOutput) {
		t.Fatalf("tx.createAssetCalls[2].Role = %q", tx.createAssetCalls[2].Role)
	}
	if len(blobs.uploadCalls) != 2 {
		t.Fatalf("len(blobs.uploadCalls) = %d, want 2", len(blobs.uploadCalls))
	}
	if got := blobs.uploadCalls[0].blobPath; got != "sessions/11111111-1111-1111-1111-111111111111/presentation-generations/22222222-2222-2222-2222-222222222222/resolved/travel-editorial-hero-image.png" {
		t.Fatalf("blobs.uploadCalls[0].blobPath = %q", got)
	}
	if !strings.Contains(blobs.uploadCalls[1].blobPath, "/outputs/") {
		t.Fatalf("blobs.uploadCalls[1].blobPath = %q", blobs.uploadCalls[1].blobPath)
	}
}

func TestCompleteGenerationCleansUpUploadedBlobsWhenGenerationBecomesTerminalBeforePersistence(t *testing.T) {
	t.Parallel()

	completedAt := time.Date(2026, 4, 17, 6, 0, 0, 0, time.UTC)
	tx := &stubWriteTx{
		lockedGeneration: repository.PresentationGeneration{
			ID:          "22222222-2222-2222-2222-222222222222",
			SessionID:   "11111111-1111-1111-1111-111111111111",
			Status:      string(StatusCompleted),
			CompletedAt: &completedAt,
		},
	}
	blobs := &stubBlobStore{}
	service := &Service{
		blobs:          blobs,
		beginWriteTxFn: func(context.Context) (writeTx, error) { return tx, nil },
		assetResolver:  newDefaultAssetResolver(blobs),
	}

	err := service.completeGeneration(
		context.Background(),
		repository.PresentationGeneration{
			ID:            "22222222-2222-2222-2222-222222222222",
			SessionID:     "11111111-1111-1111-1111-111111111111",
			ProviderName:  plannerProviderName,
			ProviderModel: "presentation-deployment",
			Status:        string(StatusRunning),
		},
		nil,
		azureopenai.Response{},
		[]byte(`{"version":"v2","themePresetId":"travel_editorial","slides":[{"layout":"chapter_divider","title":"Highlights","blocks":[{"type":"image","assetRef":"theme:hero-image"}]}]}`),
		[]byte(`{"version":"v2","slideSize":"16:9","themePresetId":"travel_editorial","slides":[{"layout":"chapter_divider","title":"Highlights","blocks":[{"type":"image","assetRef":"theme:hero-image"}]}]}`),
	)
	if err != nil {
		t.Fatalf("completeGeneration() error = %v", err)
	}
	if len(tx.createAssetCalls) != 0 {
		t.Fatalf("len(tx.createAssetCalls) = %d, want 0", len(tx.createAssetCalls))
	}
	if len(blobs.uploadCalls) != 2 {
		t.Fatalf("len(blobs.uploadCalls) = %d, want 2", len(blobs.uploadCalls))
	}
	if len(blobs.deleteCalls) != 2 {
		t.Fatalf("len(blobs.deleteCalls) = %d, want 2", len(blobs.deleteCalls))
	}
	if !strings.Contains(blobs.deleteCalls[0], "/resolved/") {
		t.Fatalf("blobs.deleteCalls[0] = %q", blobs.deleteCalls[0])
	}
	if !strings.Contains(blobs.deleteCalls[1], "/outputs/") {
		t.Fatalf("blobs.deleteCalls[1] = %q", blobs.deleteCalls[1])
	}
	if !tx.rolledBack {
		t.Fatal("expected rollback when generation is already terminal")
	}
}

func TestExecuteGenerationFailsWhenResolvedAttachmentRefCannotBeMatched(t *testing.T) {
	t.Parallel()

	reader := &stubGenerationReader{
		generation: repository.PresentationGeneration{
			ID:        "22222222-2222-2222-2222-222222222222",
			SessionID: "11111111-1111-1111-1111-111111111111",
			Prompt:    "Build a travel deck",
			Status:    string(StatusPending),
		},
		claimPendingResult: true,
	}
	client := &stubResponseClient{
		responses: []azureopenai.Response{{
			OutputText: `{
				"version": "v2",
				"slides": [
					{"layout": "cover_hero", "title": "Kyoto", "blocks": [{"type": "image", "assetRef": "attachment:missing"}]}
				]
			}`,
		}},
	}
	blobs := &stubBlobStore{}
	tx := &stubWriteTx{
		lockedGeneration: repository.PresentationGeneration{
			ID:            "22222222-2222-2222-2222-222222222222",
			SessionID:     "11111111-1111-1111-1111-111111111111",
			ProviderName:  plannerProviderName,
			ProviderModel: "presentation-deployment",
			Status:        string(StatusRunning),
		},
	}
	service := &Service{
		planner:        PlannerConfig{Deployment: "presentation-deployment"},
		blobs:          blobs,
		responses:      client,
		generationRead: reader,
		assetRead:      stubAssetReader{},
		beginWriteTxFn: func(context.Context) (writeTx, error) { return tx, nil },
		assetResolver:  newDefaultAssetResolver(blobs),
	}

	err := service.ExecuteGeneration(context.Background(), "22222222-2222-2222-2222-222222222222")
	if err == nil {
		t.Fatal("ExecuteGeneration() error = nil, want error")
	}
	if len(reader.updateCalls) != 1 {
		t.Fatalf("len(reader.updateCalls) = %d, want 1", len(reader.updateCalls))
	}
	if reader.updateCalls[0].ErrorCode != "resolved_asset_not_found" {
		t.Fatalf("reader.updateCalls[0].ErrorCode = %q", reader.updateCalls[0].ErrorCode)
	}
	if !strings.Contains(reader.updateCalls[0].ErrorMessage, "attachment:missing") {
		t.Fatalf("reader.updateCalls[0].ErrorMessage = %q", reader.updateCalls[0].ErrorMessage)
	}
}

func TestCompleteGenerationCleansUpUploadedThemeAssetsWhenResolvedAssetPersistenceFails(t *testing.T) {
	t.Parallel()

	tx := &stubWriteTx{
		lockedGeneration: repository.PresentationGeneration{
			ID:            "22222222-2222-2222-2222-222222222222",
			SessionID:     "11111111-1111-1111-1111-111111111111",
			ProviderName:  plannerProviderName,
			ProviderModel: "presentation-deployment",
			Status:        string(StatusRunning),
		},
		createAssetErr: errors.New("persist asset failed"),
	}
	blobs := &stubBlobStore{}
	service := &Service{
		blobs:          blobs,
		beginWriteTxFn: func(context.Context) (writeTx, error) { return tx, nil },
		assetResolver:  newDefaultAssetResolver(blobs),
		generationRead: &stubGenerationReader{},
	}

	err := service.completeGeneration(
		context.Background(),
		repository.PresentationGeneration{
			ID:            "22222222-2222-2222-2222-222222222222",
			SessionID:     "11111111-1111-1111-1111-111111111111",
			ProviderName:  plannerProviderName,
			ProviderModel: "presentation-deployment",
			Status:        string(StatusRunning),
		},
		nil,
		azureopenai.Response{},
		[]byte(`{"version":"v2","themePresetId":"travel_editorial","slides":[{"layout":"chapter_divider","title":"Highlights","blocks":[{"type":"image","assetRef":"theme:hero-image"}]}]}`),
		[]byte(`{"version":"v2","slideSize":"16:9","themePresetId":"travel_editorial","slides":[{"layout":"chapter_divider","title":"Highlights","blocks":[{"type":"image","assetRef":"theme:hero-image"}]}]}`),
	)
	if err == nil {
		t.Fatal("completeGeneration() error = nil, want error")
	}
	if len(blobs.deleteCalls) != 2 {
		t.Fatalf("len(blobs.deleteCalls) = %d, want 2", len(blobs.deleteCalls))
	}
	if !strings.Contains(blobs.deleteCalls[0], "/resolved/") {
		t.Fatalf("blobs.deleteCalls[0] = %q", blobs.deleteCalls[0])
	}
	if !strings.Contains(blobs.deleteCalls[1], "/outputs/") {
		t.Fatalf("blobs.deleteCalls[1] = %q", blobs.deleteCalls[1])
	}
}

func TestCompleteGenerationPersistsPlanWhenOutputPersistenceTransactionCannotStart(t *testing.T) {
	t.Parallel()

	reader := &stubGenerationReader{}
	blobs := &stubBlobStore{}
	service := &Service{
		blobs:          blobs,
		assetResolver:  newDefaultAssetResolver(blobs),
		generationRead: reader,
	}

	plannerOutputJSON := []byte(`{"version":"v1","slides":[{"layout":"title","title":"Roadmap"}]}`)
	dialectJSON := []byte(`{"version":"v1","slideSize":"16:9","slides":[{"layout":"title","title":"Roadmap"}]}`)
	err := service.completeGeneration(
		context.Background(),
		repository.PresentationGeneration{
			ID:            "22222222-2222-2222-2222-222222222222",
			SessionID:     "11111111-1111-1111-1111-111111111111",
			ProviderName:  plannerProviderName,
			ProviderModel: "presentation-deployment",
			Status:        string(StatusRunning),
		},
		nil,
		azureopenai.Response{},
		plannerOutputJSON,
		dialectJSON,
	)
	if err == nil {
		t.Fatal("completeGeneration() error = nil, want error")
	}
	if len(reader.updateCalls) != 1 {
		t.Fatalf("len(reader.updateCalls) = %d, want 1", len(reader.updateCalls))
	}
	if got := string(reader.updateCalls[0].PlannerOutputJSON); got != string(plannerOutputJSON) {
		t.Fatalf("reader.updateCalls[0].PlannerOutputJSON = %s", got)
	}
	if got := string(reader.updateCalls[0].DialectJSON); got != string(dialectJSON) {
		t.Fatalf("reader.updateCalls[0].DialectJSON = %s", got)
	}
	if len(blobs.deleteCalls) != 1 || !strings.Contains(blobs.deleteCalls[0], "/outputs/") {
		t.Fatalf("blobs.deleteCalls = %#v, want uploaded output blob cleanup", blobs.deleteCalls)
	}
}

func TestCompleteGenerationPersistsPlanWhenOutputAssetPersistenceFails(t *testing.T) {
	t.Parallel()

	reader := &stubGenerationReader{}
	tx := &stubWriteTx{
		lockedGeneration: repository.PresentationGeneration{
			ID:            "22222222-2222-2222-2222-222222222222",
			SessionID:     "11111111-1111-1111-1111-111111111111",
			ProviderName:  plannerProviderName,
			ProviderModel: "presentation-deployment",
			Status:        string(StatusRunning),
		},
		createAssetErr: errors.New("persist output asset failed"),
	}
	blobs := &stubBlobStore{}
	service := &Service{
		blobs:          blobs,
		beginWriteTxFn: func(context.Context) (writeTx, error) { return tx, nil },
		assetResolver:  newDefaultAssetResolver(blobs),
		generationRead: reader,
	}

	plannerOutputJSON := []byte(`{"version":"v1","slides":[{"layout":"title","title":"Roadmap"}]}`)
	dialectJSON := []byte(`{"version":"v1","slideSize":"16:9","slides":[{"layout":"title","title":"Roadmap"}]}`)
	err := service.completeGeneration(
		context.Background(),
		repository.PresentationGeneration{
			ID:            "22222222-2222-2222-2222-222222222222",
			SessionID:     "11111111-1111-1111-1111-111111111111",
			ProviderName:  plannerProviderName,
			ProviderModel: "presentation-deployment",
			Status:        string(StatusRunning),
		},
		nil,
		azureopenai.Response{},
		plannerOutputJSON,
		dialectJSON,
	)
	if err == nil {
		t.Fatal("completeGeneration() error = nil, want error")
	}
	if !tx.rolledBack {
		t.Fatal("expected rollback when output asset persistence fails")
	}
	if len(reader.updateCalls) != 1 {
		t.Fatalf("len(reader.updateCalls) = %d, want 1", len(reader.updateCalls))
	}
	if got := string(reader.updateCalls[0].PlannerOutputJSON); got != string(plannerOutputJSON) {
		t.Fatalf("reader.updateCalls[0].PlannerOutputJSON = %s", got)
	}
	if got := string(reader.updateCalls[0].DialectJSON); got != string(dialectJSON) {
		t.Fatalf("reader.updateCalls[0].DialectJSON = %s", got)
	}
	if len(blobs.deleteCalls) != 1 || !strings.Contains(blobs.deleteCalls[0], "/outputs/") {
		t.Fatalf("blobs.deleteCalls = %#v, want uploaded output blob cleanup", blobs.deleteCalls)
	}
}

func TestExecuteGenerationIncludesWebSearchToolWhenEnabled(t *testing.T) {
	t.Parallel()

	reader := &stubGenerationReader{
		generation: repository.PresentationGeneration{
			ID:        "22222222-2222-2222-2222-222222222222",
			SessionID: "11111111-1111-1111-1111-111111111111",
			Prompt:    "Build a market overview deck",
			Status:    string(StatusPending),
		},
		claimPendingResult: true,
	}
	client := &stubResponseClient{
		responses: []azureopenai.Response{{
			Model:      "gpt-5-presentation",
			OutputText: `{"version":"v1","slides":[{"layout":"title","title":"Market overview"}]}`,
		}},
	}
	tx := &stubWriteTx{
		lockedGeneration: repository.PresentationGeneration{
			ID:            "22222222-2222-2222-2222-222222222222",
			SessionID:     "11111111-1111-1111-1111-111111111111",
			ProviderName:  plannerProviderName,
			ProviderModel: "presentation-deployment",
			Status:        string(StatusRunning),
		},
	}

	service := &Service{
		planner: PlannerConfig{
			Deployment: "presentation-deployment",
		},
		blobs:          &stubBlobStore{},
		responses:      client,
		webSearch:      true,
		generationRead: reader,
		assetRead:      stubAssetReader{},
		beginWriteTxFn: func(context.Context) (writeTx, error) { return tx, nil },
	}

	if err := service.ExecuteGeneration(context.Background(), "22222222-2222-2222-2222-222222222222"); err != nil {
		t.Fatalf("ExecuteGeneration() error = %v", err)
	}

	if len(client.requests) != 1 {
		t.Fatalf("len(client.requests) = %d, want 1", len(client.requests))
	}
	if len(client.requests[0].Tools) != 1 {
		t.Fatalf("len(client.requests[0].Tools) = %d, want 1", len(client.requests[0].Tools))
	}
	if client.requests[0].Tools[0].Type != "web_search" {
		t.Fatalf("client.requests[0].Tools[0].Type = %q, want web_search", client.requests[0].Tools[0].Type)
	}
}

func TestNormalizePlannerOutputNormalizesV2ThemePresetFallback(t *testing.T) {
	t.Parallel()

	normalized, err := normalizePlannerOutput(`{
		"version": "v2",
		"themePresetId": "missing_preset",
		"slides": [
			{
				"layout": "cover_hero",
				"title": "Kyoto",
				"blocks": [
					{
						"type": "image",
						"assetRef": "attachment:cover"
					}
				]
			}
		]
	}`)
	if err != nil {
		t.Fatalf("normalizePlannerOutput() error = %v", err)
	}

	if got := string(normalized); got != `{"version":"v2","slideSize":"16:9","themePresetId":"general_clean","slides":[{"layout":"cover_hero","title":"Kyoto","blocks":[{"type":"image","assetRef":"attachment:cover"}]}]}` {
		t.Fatalf("normalizePlannerOutput() = %s", got)
	}
}

func TestCapabilitiesExposeThemePresets(t *testing.T) {
	t.Parallel()

	capabilities := (&Service{}).Capabilities()
	if len(capabilities.ThemePresets) < 2 {
		t.Fatalf("len(capabilities.ThemePresets) = %d, want at least 2", len(capabilities.ThemePresets))
	}
	if capabilities.ThemePresets[0].ID != presentationdialect.ThemePresetGeneralClean || !capabilities.ThemePresets[0].IsDefault {
		t.Fatalf("capabilities.ThemePresets[0] = %#v", capabilities.ThemePresets[0])
	}
	if !strings.Contains(plannerDialectInstructions, `"version": "v2"`) {
		t.Fatalf("plannerDialectInstructions = %q, want v2 guidance", plannerDialectInstructions)
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
	tx := &stubWriteTx{
		lockedGeneration: repository.PresentationGeneration{
			ID:            "22222222-2222-2222-2222-222222222222",
			SessionID:     "11111111-1111-1111-1111-111111111111",
			ProviderName:  plannerProviderName,
			ProviderModel: "presentation-deployment",
			Status:        string(StatusRunning),
		},
	}

	service := &Service{
		planner:        PlannerConfig{Deployment: "presentation-deployment"},
		responses:      client,
		generationRead: reader,
		assetRead:      stubAssetReader{},
		blobs:          &stubBlobStore{},
		beginWriteTxFn: func(context.Context) (writeTx, error) { return tx, nil },
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
	if len(tx.updateCalls) != 1 || tx.updateCalls[0].Status != string(StatusCompleted) {
		t.Fatalf("updateCalls = %#v, want completed update after repair", tx.updateCalls)
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
	if got := string(update.PlannerOutputJSON); got != "still not json" {
		t.Fatalf("update.PlannerOutputJSON = %q, want latest invalid planner output", got)
	}
	if got := string(update.DialectJSON); got != "" {
		t.Fatalf("update.DialectJSON = %q, want empty", got)
	}
}

func TestExecuteGenerationFailsWhenProviderReturnsFailedResponse(t *testing.T) {
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
		responses: []azureopenai.Response{{
			Status: "failed",
			Error: &azureopenai.ResponseError{
				Code:    "server_error",
				Message: "planner backend unavailable",
			},
		}},
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
	if update.ErrorCode != "provider_request_failed" {
		t.Fatalf("update.ErrorCode = %q, want %q", update.ErrorCode, "provider_request_failed")
	}
	if !strings.Contains(update.ErrorMessage, "server_error: planner backend unavailable") {
		t.Fatalf("update.ErrorMessage = %q, want provider response details", update.ErrorMessage)
	}
	if got := string(update.DialectJSON); got != "" {
		t.Fatalf("update.DialectJSON = %q, want empty", got)
	}
}

func TestPersistGenerationFailurePreservesDialectJSON(t *testing.T) {
	t.Parallel()

	reader := &stubGenerationReader{}
	service := &Service{
		generationRead: reader,
	}

	err := service.persistGenerationFailure(context.Background(), repository.PresentationGeneration{
		ID:            "22222222-2222-2222-2222-222222222222",
		ProviderName:  plannerProviderName,
		ProviderModel: "presentation-deployment",
		DialectJSON:   []byte(`{"version":"v1","slides":[{"layout":"title","title":"Quarterly review"}]}`),
	}, "provider_request_failed", "planner backend unavailable")
	if err != nil {
		t.Fatalf("persistGenerationFailure() error = %v", err)
	}

	if len(reader.updateCalls) != 1 {
		t.Fatalf("len(reader.updateCalls) = %d, want 1", len(reader.updateCalls))
	}
	if got := string(reader.updateCalls[0].DialectJSON); got != `{"version":"v1","slides":[{"layout":"title","title":"Quarterly review"}]}` {
		t.Fatalf("update.DialectJSON = %q", got)
	}
}

func TestExecuteGenerationFailsWithOversizedAttachmentCode(t *testing.T) {
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

	service := &Service{
		blobs:          &stubBlobStore{data: map[string][]byte{"blob://large": {1, 2, 3}}},
		responses:      &stubResponseClient{},
		generationRead: reader,
		assetRead: stubAssetReader{assets: []repository.PresentationGenerationAsset{{
			ID:        "asset-large",
			Role:      string(AssetRoleInput),
			BlobPath:  "blob://large",
			MediaType: InputMediaTypePDF,
			Filename:  "large.pdf",
			SizeBytes: defaultMaxAttachmentBytes + 1,
		}}},
	}

	err := service.ExecuteGeneration(context.Background(), "22222222-2222-2222-2222-222222222222")
	if err == nil {
		t.Fatal("ExecuteGeneration() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "prepare presentation planner request") {
		t.Fatalf("ExecuteGeneration() error = %q, want request preparation failure", err.Error())
	}
	if len(reader.updateCalls) != 1 {
		t.Fatalf("len(reader.updateCalls) = %d, want 1", len(reader.updateCalls))
	}
	if reader.updateCalls[0].ErrorCode != "input_asset_too_large" {
		t.Fatalf("update.ErrorCode = %q, want %q", reader.updateCalls[0].ErrorCode, "input_asset_too_large")
	}
}

func TestExecuteGenerationMapsBlobDownloadLimitErrorToOversizedAttachmentCode(t *testing.T) {
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

	service := &Service{
		blobs: &stubBlobStore{
			downloadErrs: map[string]error{
				"blob://large": blobstorage.MaxBytesExceededError{BlobPath: "blob://large", MaxBytes: defaultMaxAttachmentBytes},
			},
		},
		responses:      &stubResponseClient{},
		generationRead: reader,
		assetRead: stubAssetReader{assets: []repository.PresentationGenerationAsset{{
			ID:        "asset-large",
			Role:      string(AssetRoleInput),
			BlobPath:  "blob://large",
			MediaType: InputMediaTypePDF,
			Filename:  "large.pdf",
			SizeBytes: defaultMaxAttachmentBytes,
		}}},
	}

	err := service.ExecuteGeneration(context.Background(), "22222222-2222-2222-2222-222222222222")
	if err == nil {
		t.Fatal("ExecuteGeneration() error = nil, want error")
	}
	if len(reader.updateCalls) != 1 {
		t.Fatalf("len(reader.updateCalls) = %d, want 1", len(reader.updateCalls))
	}
	if reader.updateCalls[0].ErrorCode != "input_asset_too_large" {
		t.Fatalf("update.ErrorCode = %q, want %q", reader.updateCalls[0].ErrorCode, "input_asset_too_large")
	}
}

func TestExecuteGenerationFailsWithUnsupportedMediaTypeCode(t *testing.T) {
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

	service := &Service{
		blobs:          &stubBlobStore{data: map[string][]byte{"blob://doc": {1, 2, 3}}},
		responses:      &stubResponseClient{},
		generationRead: reader,
		assetRead: stubAssetReader{assets: []repository.PresentationGenerationAsset{{
			ID:        "asset-doc",
			Role:      string(AssetRoleInput),
			BlobPath:  "blob://doc",
			MediaType: "text/plain",
			Filename:  "notes.txt",
			SizeBytes: 3,
		}}},
	}

	err := service.ExecuteGeneration(context.Background(), "22222222-2222-2222-2222-222222222222")
	if err == nil {
		t.Fatal("ExecuteGeneration() error = nil, want error")
	}
	if len(reader.updateCalls) != 1 {
		t.Fatalf("len(reader.updateCalls) = %d, want 1", len(reader.updateCalls))
	}
	if reader.updateCalls[0].ErrorCode != "input_asset_unsupported_media_type" {
		t.Fatalf("update.ErrorCode = %q, want %q", reader.updateCalls[0].ErrorCode, "input_asset_unsupported_media_type")
	}
}

func TestPrepareAttachmentInputRejectsUnsupportedMediaTypeWithoutDownloading(t *testing.T) {
	t.Parallel()

	blobs := &stubBlobStore{
		data: map[string][]byte{
			"blob://doc": {1, 2, 3},
		},
	}
	service := &Service{blobs: blobs}

	_, err := service.prepareAttachmentInput(context.Background(), repository.PresentationGenerationAsset{
		ID:        "asset-doc",
		Role:      string(AssetRoleInput),
		BlobPath:  "blob://doc",
		MediaType: "text/plain",
		Filename:  "notes.txt",
		SizeBytes: 3,
	})
	if err == nil {
		t.Fatal("prepareAttachmentInput() error = nil, want error")
	}
	if !errors.Is(err, errPlannerInputAssetUnsupportedMediaType) {
		t.Fatalf("prepareAttachmentInput() error = %v, want unsupported media type", err)
	}
	if len(blobs.downloadCalls) != 0 {
		t.Fatalf("len(blobs.downloadCalls) = %d, want 0", len(blobs.downloadCalls))
	}
}

func TestPrepareAttachmentInputRejectsOversizedAsset(t *testing.T) {
	t.Parallel()

	service := &Service{
		blobs: &stubBlobStore{
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
	generation         repository.PresentationGeneration
	err                error
	claimPendingResult bool
	claimPendingErr    error
	updateCalls        []repository.UpdatePresentationGenerationStateParams
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
	assets            []repository.PresentationGenerationAsset
	assetByID         repository.PresentationGenerationAsset
	expectedSessionID string
	err               error
}

func (s stubAssetReader) GetByIDAndSession(_ context.Context, assetID string, sessionID string) (repository.PresentationGenerationAsset, error) {
	if s.err != nil {
		return repository.PresentationGenerationAsset{}, s.err
	}
	if s.assetByID.ID != "" {
		if s.assetByID.ID == assetID && (s.expectedSessionID == "" || s.expectedSessionID == sessionID) {
			return s.assetByID, nil
		}
		return repository.PresentationGenerationAsset{}, repository.ErrNotFound
	}
	for _, asset := range s.assets {
		if asset.ID == assetID {
			if s.expectedSessionID != "" && s.expectedSessionID != sessionID {
				return repository.PresentationGenerationAsset{}, repository.ErrNotFound
			}
			return asset, nil
		}
	}
	return repository.PresentationGenerationAsset{}, repository.ErrNotFound
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
	data          map[string][]byte
	downloadErrs  map[string]error
	downloadCalls []string
	uploadCalls   []stubUploadCall
	deleteCalls   []string
}

type stubUploadCall struct {
	blobPath    string
	data        []byte
	contentType string
}

func (s *stubBlobStore) Upload(_ context.Context, blobPath string, data []byte, contentType string) error {
	s.uploadCalls = append(s.uploadCalls, stubUploadCall{
		blobPath:    blobPath,
		data:        append([]byte(nil), data...),
		contentType: contentType,
	})
	if s.data == nil {
		s.data = map[string][]byte{}
	}
	s.data[blobPath] = append([]byte(nil), data...)
	return nil
}

func (s *stubBlobStore) Delete(_ context.Context, blobPath string) error {
	s.deleteCalls = append(s.deleteCalls, blobPath)
	delete(s.data, blobPath)
	return nil
}

func (s *stubBlobStore) Download(_ context.Context, blobPath string) ([]byte, error) {
	s.downloadCalls = append(s.downloadCalls, blobPath)
	if err, ok := s.downloadErrs[blobPath]; ok {
		return nil, err
	}
	data, ok := s.data[blobPath]
	if !ok {
		return nil, context.Canceled
	}

	return data, nil
}

type stubWriteTx struct {
	session                repository.Session
	generation             repository.PresentationGeneration
	lockedGeneration       repository.PresentationGeneration
	createGenerationErr    error
	createAssetErr         error
	lockErr                error
	updateErr              error
	commitErr              error
	rollbackErr            error
	createGenerationParams repository.CreatePresentationGenerationParams
	createAssetCalls       []repository.CreatePresentationGenerationAssetParams
	updateCalls            []repository.UpdatePresentationGenerationStateParams
	committed              bool
	rolledBack             bool
}

func (s *stubWriteTx) GetSession(context.Context, string) (repository.Session, error) {
	if s.session.ID == "" {
		return repository.Session{}, repository.ErrNotFound
	}
	return s.session, nil
}

func (s *stubWriteTx) CreateGeneration(_ context.Context, params repository.CreatePresentationGenerationParams) (repository.PresentationGeneration, error) {
	if s.createGenerationErr != nil {
		return repository.PresentationGeneration{}, s.createGenerationErr
	}
	s.createGenerationParams = params
	if s.generation.ID != "" {
		generation := s.generation
		generation.Prompt = params.Prompt
		generation.PlannerOutputJSON = params.PlannerOutputJSON
		generation.DialectJSON = params.DialectJSON
		generation.Status = params.Status
		return generation, nil
	}
	return repository.PresentationGeneration{
		ID:                "generated-presentation",
		SessionID:         params.SessionID,
		Prompt:            params.Prompt,
		PlannerOutputJSON: params.PlannerOutputJSON,
		DialectJSON:       params.DialectJSON,
		Status:            params.Status,
	}, nil
}

func (s *stubWriteTx) CreateGenerationAsset(_ context.Context, params repository.CreatePresentationGenerationAssetParams) (repository.PresentationGenerationAsset, error) {
	if s.createAssetErr != nil {
		return repository.PresentationGenerationAsset{}, s.createAssetErr
	}
	s.createAssetCalls = append(s.createAssetCalls, params)
	return repository.PresentationGenerationAsset{
		ID:            fmt.Sprintf("asset-%d", len(s.createAssetCalls)),
		GenerationID:  params.GenerationID,
		Role:          params.Role,
		AssetRef:      params.AssetRef,
		SourceType:    params.SourceType,
		SourceAssetID: params.SourceAssetID,
		SourceRef:     params.SourceRef,
		SortOrder:     params.SortOrder,
		BlobPath:      params.BlobPath,
		MediaType:     params.MediaType,
		Filename:      params.Filename,
		SizeBytes:     params.SizeBytes,
		Sha256:        params.Sha256,
		CreatedAt:     time.Now().UTC(),
	}, nil
}

func (s *stubWriteTx) LockGenerationForUpdate(context.Context, string) (repository.PresentationGeneration, error) {
	if s.lockErr != nil {
		return repository.PresentationGeneration{}, s.lockErr
	}
	if s.lockedGeneration.ID != "" {
		return s.lockedGeneration, nil
	}
	return s.generation, nil
}

func (s *stubWriteTx) UpdateGenerationState(_ context.Context, params repository.UpdatePresentationGenerationStateParams) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	s.updateCalls = append(s.updateCalls, params)
	return nil
}

func (s *stubWriteTx) Commit(context.Context) error {
	if s.commitErr != nil {
		return s.commitErr
	}
	s.committed = true
	return nil
}

func (s *stubWriteTx) Rollback(context.Context) error {
	s.rolledBack = true
	return s.rollbackErr
}

func testPNGData() []byte {
	return []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
		0x89, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x44, 0x41,
		0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00,
		0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00,
		0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae,
		0x42, 0x60, 0x82,
	}
}

func (s *stubBlobStore) DownloadWithinLimit(ctx context.Context, blobPath string, maxBytes int64) ([]byte, error) {
	data, err := s.Download(ctx, blobPath)
	if err != nil {
		return nil, err
	}
	if maxBytes > 0 && int64(len(data)) > maxBytes {
		return nil, blobstorage.MaxBytesExceededError{BlobPath: blobPath, MaxBytes: maxBytes}
	}

	return data, nil
}
