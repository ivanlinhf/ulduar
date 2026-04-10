package imagegen

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/ivanlin/ulduar/apps/backend/internal/repository"
)

func TestSupportedResolutionsReturnsCatalogCopy(t *testing.T) {
	t.Parallel()

	resolutions := SupportedResolutions()
	if len(resolutions) != 7 {
		t.Fatalf("len(resolutions) = %d, want 7", len(resolutions))
	}
	if resolutions[0].Key != "1024x1024" || resolutions[6].Key != "1024x1536" {
		t.Fatalf("unexpected resolution catalog: %+v", resolutions)
	}

	resolutions[0].Key = "changed"
	if SupportedResolutions()[0].Key != "1024x1024" {
		t.Fatal("SupportedResolutions() did not return a copy")
	}
}

func TestValidateCreateGenerationParams(t *testing.T) {
	t.Parallel()

	validPNG := []byte{
		0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n',
		0x00, 0x00, 0x00, 0x0d, 'I', 'H', 'D', 'R',
		0x00, 0x00, 0x00, 0x01,
		0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00,
	}
	validGIF := []byte("GIF89a\x01\x00\x01\x00\x00\x00\x00")

	tests := []struct {
		name       string
		params     CreateGenerationParams
		wantStatus int
		wantSubstr string
	}{
		{
			name: "empty prompt",
			params: CreateGenerationParams{
				SessionID:     "11111111-1111-1111-1111-111111111111",
				Mode:          ModeTextToImage,
				ResolutionKey: "1024x1024",
			},
			wantStatus: http.StatusBadRequest,
			wantSubstr: "prompt is required",
		},
		{
			name: "invalid mode",
			params: CreateGenerationParams{
				SessionID:     "11111111-1111-1111-1111-111111111111",
				Mode:          Mode("other"),
				Prompt:        "draw",
				ResolutionKey: "1024x1024",
			},
			wantStatus: http.StatusBadRequest,
			wantSubstr: "mode must be one of",
		},
		{
			name: "unsupported resolution",
			params: CreateGenerationParams{
				SessionID:     "11111111-1111-1111-1111-111111111111",
				Mode:          ModeTextToImage,
				Prompt:        "draw",
				ResolutionKey: "800x800",
			},
			wantStatus: http.StatusBadRequest,
			wantSubstr: "resolution must be one of",
		},
		{
			name: "text_to_image rejects references",
			params: CreateGenerationParams{
				SessionID:     "11111111-1111-1111-1111-111111111111",
				Mode:          ModeTextToImage,
				Prompt:        "draw",
				ResolutionKey: "1024x1024",
				ReferenceImages: []InputAssetUpload{{
					Filename: "ref.png",
					Data:     validPNG,
				}},
			},
			wantStatus: http.StatusBadRequest,
			wantSubstr: "only supported for image_edit",
		},
		{
			name: "image_edit requires reference",
			params: CreateGenerationParams{
				SessionID:     "11111111-1111-1111-1111-111111111111",
				Mode:          ModeImageEdit,
				Prompt:        "draw",
				ResolutionKey: "1024x1024",
			},
			wantStatus: http.StatusBadRequest,
			wantSubstr: "requires at least one reference image",
		},
		{
			name: "too many references",
			params: CreateGenerationParams{
				SessionID:     "11111111-1111-1111-1111-111111111111",
				Mode:          ModeImageEdit,
				Prompt:        "draw",
				ResolutionKey: "1024x1024",
				ReferenceImages: []InputAssetUpload{
					{Filename: "1.png", Data: validPNG},
					{Filename: "2.png", Data: validPNG},
					{Filename: "3.png", Data: validPNG},
					{Filename: "4.png", Data: validPNG},
					{Filename: "5.png", Data: validPNG},
				},
			},
			wantStatus: http.StatusBadRequest,
			wantSubstr: "too many reference images",
		},
		{
			name: "rejects gif input",
			params: CreateGenerationParams{
				SessionID:     "11111111-1111-1111-1111-111111111111",
				Mode:          ModeImageEdit,
				Prompt:        "draw",
				ResolutionKey: "1024x1024",
				ReferenceImages: []InputAssetUpload{{
					Filename: "ref.gif",
					Data:     validGIF,
				}},
			},
			wantStatus: http.StatusUnsupportedMediaType,
			wantSubstr: `unsupported media type "image/gif"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, _, _, err := validateCreateGenerationParams(tt.params, DefaultMaxReferenceImageBytes)
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			validationErr, ok := err.(ValidationError)
			if !ok {
				t.Fatalf("expected ValidationError, got %T", err)
			}
			if validationErr.StatusCode != tt.wantStatus {
				t.Fatalf("validationErr.StatusCode = %d, want %d", validationErr.StatusCode, tt.wantStatus)
			}
			if !strings.Contains(validationErr.Message, tt.wantSubstr) {
				t.Fatalf("validationErr.Message = %q, want substring %q", validationErr.Message, tt.wantSubstr)
			}
		})
	}
}

func TestValidateCreateGenerationParamsRejectsOversizedReferenceImage(t *testing.T) {
	t.Parallel()

	_, _, _, err := validateCreateGenerationParams(CreateGenerationParams{
		SessionID:     "11111111-1111-1111-1111-111111111111",
		Mode:          ModeImageEdit,
		Prompt:        "draw",
		ResolutionKey: "1024x1024",
		ReferenceImages: []InputAssetUpload{{
			Filename: "ref.png",
			Data: []byte{
				0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n',
				0x00, 0x00, 0x00, 0x0d, 'I', 'H', 'D', 'R',
				0x00, 0x00, 0x00, 0x01,
				0x00, 0x00, 0x00, 0x01,
				0x08, 0x02, 0x00, 0x00, 0x00,
			},
		}},
	}, 8)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	validationErr, ok := err.(ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if validationErr.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("validationErr.StatusCode = %d, want %d", validationErr.StatusCode, http.StatusRequestEntityTooLarge)
	}
	if !strings.Contains(validationErr.Message, "exceeds 8 bytes") {
		t.Fatalf("validationErr.Message = %q", validationErr.Message)
	}
}

func TestNewServiceDefaultsAndOverridesOptions(t *testing.T) {
	t.Parallel()

	defaultService := NewService(nil, nil)
	if defaultService.maxReferenceImageBytes != DefaultMaxReferenceImageBytes {
		t.Fatalf("defaultService.maxReferenceImageBytes = %d, want %d", defaultService.maxReferenceImageBytes, DefaultMaxReferenceImageBytes)
	}

	configuredService := NewService(nil, nil, ServiceOptions{MaxReferenceImageBytes: 1234})
	if configuredService.maxReferenceImageBytes != 1234 {
		t.Fatalf("configuredService.maxReferenceImageBytes = %d, want 1234", configuredService.maxReferenceImageBytes)
	}
}

func TestNewServicePanicsOnMultipleOptions(t *testing.T) {
	t.Parallel()

	defer func() {
		if recovered := recover(); recovered == nil {
			t.Fatal("expected panic, got nil")
		}
	}()

	_ = NewService(nil, nil, ServiceOptions{}, ServiceOptions{})
}

func TestBuildBlobPaths(t *testing.T) {
	t.Parallel()

	asset := preparedAsset{
		Filename: "ref.png",
		SHA256:   "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	}

	inputPath := buildInputBlobPath("session-1", "generation-1", 0, asset)
	if inputPath != "sessions/session-1/image-generations/generation-1/inputs/01-0123456789abcdef-ref.png" {
		t.Fatalf("inputPath = %q", inputPath)
	}

	outputPath := buildOutputBlobPath("session-1", "generation-1", "../output.png")
	if outputPath != "sessions/session-1/image-generations/generation-1/outputs/output.png" {
		t.Fatalf("outputPath = %q", outputPath)
	}
}

func TestCreatePendingGenerationPersistsGenerationAndUploadsInputs(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 10, 2, 0, 0, 0, time.UTC)
	blobStore := &stubBlobStore{}
	tx := &stubWriteTx{
		session: repository.Session{ID: "11111111-1111-1111-1111-111111111111"},
		generation: repository.ImageGeneration{
			ID:                  "22222222-2222-2222-2222-222222222222",
			SessionID:           "11111111-1111-1111-1111-111111111111",
			Mode:                "image_edit",
			Prompt:              "make it watercolor",
			ResolutionKey:       "1152x896",
			Width:               1152,
			Height:              896,
			RequestedImageCount: 1,
			Status:              "pending",
			CreatedAt:           now,
		},
	}

	service := &Service{
		blobs: blobStore,
		beginWriteTxFn: func(context.Context) (writeTx, error) {
			return tx, nil
		},
	}

	view, err := service.CreatePendingGeneration(context.Background(), CreateGenerationParams{
		SessionID:     "11111111-1111-1111-1111-111111111111",
		Mode:          ModeImageEdit,
		Prompt:        "  make it watercolor  ",
		ResolutionKey: "1152x896",
		ReferenceImages: []InputAssetUpload{{
			Filename: "../ref image.png",
			Data: []byte{
				0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n',
				0x00, 0x00, 0x00, 0x0d, 'I', 'H', 'D', 'R',
				0x00, 0x00, 0x00, 0x01,
				0x00, 0x00, 0x00, 0x01,
				0x08, 0x02, 0x00, 0x00, 0x00,
			},
		}},
	})
	if err != nil {
		t.Fatalf("CreatePendingGeneration() error = %v", err)
	}

	if len(tx.createdGenerations) != 1 {
		t.Fatalf("len(tx.createdGenerations) = %d, want 1", len(tx.createdGenerations))
	}
	if tx.createdGenerations[0].Prompt != "make it watercolor" {
		t.Fatalf("prompt = %q", tx.createdGenerations[0].Prompt)
	}
	if tx.createdGenerations[0].RequestedImageCount != 1 {
		t.Fatalf("requested image count = %d", tx.createdGenerations[0].RequestedImageCount)
	}
	if tx.createdGenerations[0].ProviderName != "" || tx.createdGenerations[0].ProviderModel != "" {
		t.Fatalf("expected blank provider metadata, got %+v", tx.createdGenerations[0])
	}
	if len(tx.createdAssets) != 1 {
		t.Fatalf("len(tx.createdAssets) = %d, want 1", len(tx.createdAssets))
	}
	if tx.createdAssets[0].Role != "input" {
		t.Fatalf("asset role = %q", tx.createdAssets[0].Role)
	}
	if !strings.HasPrefix(tx.createdAssets[0].BlobPath, "sessions/11111111-1111-1111-1111-111111111111/image-generations/22222222-2222-2222-2222-222222222222/inputs/") {
		t.Fatalf("blob path = %q", tx.createdAssets[0].BlobPath)
	}
	if len(blobStore.uploads) != 1 || blobStore.uploads[0].contentType != "image/png" {
		t.Fatalf("uploads = %+v", blobStore.uploads)
	}
	if !tx.committed {
		t.Fatal("expected transaction commit")
	}
	if tx.rolledBack {
		t.Fatal("did not expect rollback")
	}
	if view.Generation.OutputImageCount != 1 {
		t.Fatalf("view.Generation.OutputImageCount = %d", view.Generation.OutputImageCount)
	}
	if len(view.Assets) != 1 || view.Assets[0].Filename != "ref-image.png" {
		t.Fatalf("view.Assets = %+v", view.Assets)
	}
}

func TestCreatePendingGenerationCleansUpUploadsOnPersistenceFailure(t *testing.T) {
	t.Parallel()

	blobStore := &stubBlobStore{}
	tx := &stubWriteTx{
		session: repository.Session{ID: "11111111-1111-1111-1111-111111111111"},
		generation: repository.ImageGeneration{
			ID:                  "22222222-2222-2222-2222-222222222222",
			SessionID:           "11111111-1111-1111-1111-111111111111",
			Mode:                "image_edit",
			Prompt:              "edit",
			ResolutionKey:       "1024x1024",
			Width:               1024,
			Height:              1024,
			RequestedImageCount: 1,
			Status:              "pending",
			CreatedAt:           time.Now().UTC(),
		},
		createAssetErr: errors.New("insert failed"),
	}

	service := &Service{
		blobs: blobStore,
		beginWriteTxFn: func(context.Context) (writeTx, error) {
			return tx, nil
		},
	}

	_, err := service.CreatePendingGeneration(context.Background(), CreateGenerationParams{
		SessionID:     "11111111-1111-1111-1111-111111111111",
		Mode:          ModeImageEdit,
		Prompt:        "edit",
		ResolutionKey: "1024x1024",
		ReferenceImages: []InputAssetUpload{{
			Filename: "ref.png",
			Data: []byte{
				0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n',
				0x00, 0x00, 0x00, 0x0d, 'I', 'H', 'D', 'R',
				0x00, 0x00, 0x00, 0x01,
				0x00, 0x00, 0x00, 0x01,
				0x08, 0x02, 0x00, 0x00, 0x00,
			},
		}},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !tx.rolledBack {
		t.Fatal("expected rollback")
	}
	if tx.committed {
		t.Fatal("did not expect commit")
	}
	if len(blobStore.deletedPaths) != 1 {
		t.Fatalf("len(blobStore.deletedPaths) = %d, want 1", len(blobStore.deletedPaths))
	}
	if blobStore.deletedPaths[0] != blobStore.uploads[0].blobPath {
		t.Fatalf("deleted path = %q, uploaded path = %q", blobStore.deletedPaths[0], blobStore.uploads[0].blobPath)
	}
}

func TestGetGenerationAssemblesSessionScopedView(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, 4, 10, 2, 30, 0, 0, time.UTC)
	service := &Service{
		generationRead: stubGenerationReader{
			generation: repository.ImageGeneration{
				ID:                  "22222222-2222-2222-2222-222222222222",
				SessionID:           "11111111-1111-1111-1111-111111111111",
				Mode:                "text_to_image",
				Prompt:              "draw a lighthouse",
				ResolutionKey:       "1536x1024",
				Width:               1536,
				Height:              1024,
				RequestedImageCount: 1,
				Status:              "pending",
				CreatedAt:           createdAt,
			},
		},
		assetRead: stubAssetReader{
			assets: []repository.ImageGenerationAsset{{
				ID:           "33333333-3333-3333-3333-333333333333",
				GenerationID: "22222222-2222-2222-2222-222222222222",
				Role:         "output",
				SortOrder:    0,
				BlobPath:     "sessions/s/image-generations/g/outputs/output.png",
				MediaType:    "image/png",
				Filename:     "output.png",
				SizeBytes:    2048,
				Sha256:       "abc",
				CreatedAt:    createdAt,
			}},
		},
	}

	view, err := service.GetGeneration(
		context.Background(),
		"11111111-1111-1111-1111-111111111111",
		"22222222-2222-2222-2222-222222222222",
	)
	if err != nil {
		t.Fatalf("GetGeneration() error = %v", err)
	}

	if view.Generation.Mode != ModeTextToImage {
		t.Fatalf("view.Generation.Mode = %q", view.Generation.Mode)
	}
	if view.Generation.Resolution.Key != "1536x1024" {
		t.Fatalf("view.Generation.Resolution = %+v", view.Generation.Resolution)
	}
	if len(view.Assets) != 1 || view.Assets[0].Role != AssetRoleOutput {
		t.Fatalf("view.Assets = %+v", view.Assets)
	}
}

type stubBlobStore struct {
	uploads      []stubBlobUpload
	deletedPaths []string
}

type stubBlobUpload struct {
	blobPath    string
	contentType string
}

func (s *stubBlobStore) Upload(ctx context.Context, blobPath string, data []byte, contentType string) error {
	s.uploads = append(s.uploads, stubBlobUpload{
		blobPath:    blobPath,
		contentType: contentType,
	})
	return nil
}

func (s *stubBlobStore) Delete(ctx context.Context, blobPath string) error {
	s.deletedPaths = append(s.deletedPaths, blobPath)
	return nil
}

type stubWriteTx struct {
	session             repository.Session
	getSessionErr       error
	generation          repository.ImageGeneration
	createGenerationErr error
	createAssetErr      error
	createdGenerations  []repository.CreateImageGenerationParams
	createdAssets       []repository.CreateImageGenerationAssetParams
	committed           bool
	rolledBack          bool
}

func (s *stubWriteTx) GetSession(ctx context.Context, sessionID string) (repository.Session, error) {
	if s.getSessionErr != nil {
		return repository.Session{}, s.getSessionErr
	}
	return s.session, nil
}

func (s *stubWriteTx) CreateGeneration(ctx context.Context, params repository.CreateImageGenerationParams) (repository.ImageGeneration, error) {
	if s.createGenerationErr != nil {
		return repository.ImageGeneration{}, s.createGenerationErr
	}
	s.createdGenerations = append(s.createdGenerations, params)
	return s.generation, nil
}

func (s *stubWriteTx) CreateGenerationAsset(ctx context.Context, params repository.CreateImageGenerationAssetParams) (repository.ImageGenerationAsset, error) {
	if s.createAssetErr != nil {
		return repository.ImageGenerationAsset{}, s.createAssetErr
	}
	s.createdAssets = append(s.createdAssets, params)
	return repository.ImageGenerationAsset{
		ID:           "asset-id",
		GenerationID: params.GenerationID,
		Role:         params.Role,
		SortOrder:    params.SortOrder,
		BlobPath:     params.BlobPath,
		MediaType:    params.MediaType,
		Filename:     params.Filename,
		SizeBytes:    params.SizeBytes,
		Sha256:       params.Sha256,
		Width:        params.Width,
		Height:       params.Height,
		CreatedAt:    time.Now().UTC(),
	}, nil
}

func (s *stubWriteTx) Commit(ctx context.Context) error {
	s.committed = true
	return nil
}

func (s *stubWriteTx) Rollback(ctx context.Context) error {
	s.rolledBack = true
	return nil
}

type stubGenerationReader struct {
	generation repository.ImageGeneration
	err        error
}

func (s stubGenerationReader) GetByIDAndSession(ctx context.Context, generationID string, sessionID string) (repository.ImageGeneration, error) {
	if s.err != nil {
		return repository.ImageGeneration{}, s.err
	}
	return s.generation, nil
}

type stubAssetReader struct {
	assets []repository.ImageGenerationAsset
	err    error
}

func (s stubAssetReader) ListByGenerationAndSession(ctx context.Context, generationID string, sessionID string) ([]repository.ImageGenerationAsset, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.assets, nil
}
