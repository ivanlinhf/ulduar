package imagegen

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ivanlin/ulduar/apps/backend/internal/imageprovider"
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

func TestReadBytesWithinLimit(t *testing.T) {
	t.Parallel()

	t.Run("within limit", func(t *testing.T) {
		t.Parallel()

		data, err := readBytesWithinLimit(bytes.NewBufferString("abc"), 3)
		if err != nil {
			t.Fatalf("readBytesWithinLimit() error = %v", err)
		}
		if string(data) != "abc" {
			t.Fatalf("string(data) = %q, want %q", string(data), "abc")
		}
	})

	t.Run("exceeds limit", func(t *testing.T) {
		t.Parallel()

		_, err := readBytesWithinLimit(bytes.NewBufferString("abcd"), 3)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "content exceeds 3 bytes") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("reader error", func(t *testing.T) {
		t.Parallel()

		_, err := readBytesWithinLimit(errorReader{}, 3)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "read failed") {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestValidateOutputImageURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr string
	}{
		{
			name: "accepts https url",
			raw:  "https://cdn.example.com/output.png",
			want: "https://cdn.example.com/output.png",
		},
		{
			name:    "rejects non https",
			raw:     "http://cdn.example.com/output.png",
			wantErr: "must use HTTPS",
		},
		{
			name:    "rejects userinfo",
			raw:     "https://user:pass@cdn.example.com/output.png",
			wantErr: "invalid",
		},
		{
			name:    "rejects localhost",
			raw:     "https://localhost/output.png",
			wantErr: "host is not allowed",
		},
		{
			name:    "rejects link local ip",
			raw:     "https://169.254.169.254/output.png",
			wantErr: "host is not allowed",
		},
		{
			name:    "rejects cgnat ip",
			raw:     "https://100.64.0.1/output.png",
			wantErr: "host is not allowed",
		},
		{
			name:    "rejects relative url",
			raw:     "/output.png",
			wantErr: "absolute HTTPS URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := validateOutputImageURL(tt.raw)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want substring %q", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("validateOutputImageURL() error = %v", err)
			}
			if got.String() != tt.want {
				t.Fatalf("validateOutputImageURL() = %q, want %q", got.String(), tt.want)
			}
		})
	}
}

func TestValidateOutputImageHostRejectsResolvedRestrictedAddress(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ip   string
	}{
		{name: "private", ip: "10.0.0.7"},
		{name: "cgnat", ip: "100.64.0.1"},
		{name: "loopback", ip: "127.0.0.1"},
		{name: "link local unicast", ip: "169.254.1.10"},
		{name: "multicast", ip: "224.0.0.1"},
		{name: "unspecified", ip: "0.0.0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateOutputImageHost(context.Background(), stubHostnameResolver{
				lookupIPAddr: func(ctx context.Context, host string) ([]net.IPAddr, error) {
					if host != "cdn.example.com" {
						t.Fatalf("host = %q", host)
					}
					return []net.IPAddr{{IP: net.ParseIP(tt.ip)}}, nil
				},
			}, "cdn.example.com")
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), "host is not allowed") {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestEffectiveOutputResolver(t *testing.T) {
	t.Parallel()

	called := false
	custom := stubHostnameResolver{
		lookupIPAddr: func(ctx context.Context, host string) ([]net.IPAddr, error) {
			called = true
			if host != "cdn.example.com" {
				t.Fatalf("host = %q", host)
			}
			return nil, nil
		},
	}
	got := effectiveOutputResolver(custom)
	if _, ok := got.(stubHostnameResolver); !ok {
		t.Fatalf("effectiveOutputResolver(custom) did not return the provided resolver type")
	}
	if _, err := got.LookupIPAddr(context.Background(), "cdn.example.com"); err != nil {
		t.Fatalf("got.LookupIPAddr() error = %v", err)
	}
	if !called {
		t.Fatal("expected custom resolver to be called")
	}
	if got := effectiveOutputResolver(nil); got != net.DefaultResolver {
		t.Fatalf("effectiveOutputResolver(nil) = %#v, want net.DefaultResolver", got)
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
		generationRead: &stubGenerationReader{
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

func TestExecuteGenerationCompletesTextToImageAndPersistsOutput(t *testing.T) {
	t.Parallel()

	provider := &stubImageProvider{
		name:  "azure_foundry",
		model: "FLUX.2-pro",
		generateFn: func(context.Context, imageprovider.GenerateRequest) (imageprovider.GenerateResult, error) {
			return imageprovider.GenerateResult{
				Images: []imageprovider.OutputImage{{
					Data:      slices.Clone(testPNGData()),
					MediaType: "image/png",
				}},
			}, nil
		},
	}
	blobStore := &stubBlobStore{}
	generationRepo := &stubGenerationReader{
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
			CreatedAt:           time.Now().UTC(),
		},
	}
	tx := &stubWriteTx{
		lockGeneration: repository.ImageGeneration{
			ID:                  generationRepo.generation.ID,
			SessionID:           generationRepo.generation.SessionID,
			Mode:                generationRepo.generation.Mode,
			Prompt:              generationRepo.generation.Prompt,
			ResolutionKey:       generationRepo.generation.ResolutionKey,
			Width:               generationRepo.generation.Width,
			Height:              generationRepo.generation.Height,
			RequestedImageCount: generationRepo.generation.RequestedImageCount,
			ProviderName:        "azure_foundry",
			ProviderModel:       "FLUX.2-pro",
			Status:              "running",
			CreatedAt:           generationRepo.generation.CreatedAt,
		},
	}
	service := &Service{
		blobs:            blobStore,
		provider:         provider,
		outputHTTPClient: &http.Client{Timeout: time.Second},
		beginWriteTxFn:   func(context.Context) (writeTx, error) { return tx, nil },
		generationRead:   generationRepo,
		assetRead:        stubAssetReader{},
	}

	if err := service.ExecuteGeneration(context.Background(), generationRepo.generation.ID); err != nil {
		t.Fatalf("ExecuteGeneration() error = %v", err)
	}

	if len(provider.generateRequests) != 1 {
		t.Fatalf("len(provider.generateRequests) = %d, want 1", len(provider.generateRequests))
	}
	req := provider.generateRequests[0]
	if req.Mode != imageprovider.ModeTextToImage || req.Prompt != "draw a lighthouse" {
		t.Fatalf("provider request = %+v", req)
	}
	if req.Width != 1536 || req.Height != 1024 || req.OutputFormat != "png" || req.NumImages != 1 {
		t.Fatalf("provider request = %+v", req)
	}
	if len(generationRepo.updatedStates) != 1 || generationRepo.updatedStates[0].Status != "running" {
		t.Fatalf("generationRepo.updatedStates = %+v", generationRepo.updatedStates)
	}
	if generationRepo.updatedStates[0].ProviderName != "azure_foundry" || generationRepo.updatedStates[0].ProviderModel != "FLUX.2-pro" {
		t.Fatalf("running state metadata = %+v", generationRepo.updatedStates[0])
	}
	if len(blobStore.uploads) != 1 {
		t.Fatalf("len(blobStore.uploads) = %d, want 1", len(blobStore.uploads))
	}
	if !strings.Contains(blobStore.uploads[0].blobPath, "/outputs/output.png") {
		t.Fatalf("blobStore.uploads[0].blobPath = %q", blobStore.uploads[0].blobPath)
	}
	if len(tx.createdAssets) != 1 || tx.createdAssets[0].Role != "output" {
		t.Fatalf("tx.createdAssets = %+v", tx.createdAssets)
	}
	if len(tx.updatedStates) != 1 || tx.updatedStates[0].Status != "completed" {
		t.Fatalf("tx.updatedStates = %+v", tx.updatedStates)
	}
	if tx.updatedStates[0].ProviderName != "azure_foundry" || tx.updatedStates[0].ProviderModel != "FLUX.2-pro" {
		t.Fatalf("completed state metadata = %+v", tx.updatedStates[0])
	}
	if !tx.committed {
		t.Fatal("expected transaction commit")
	}
}

func TestExecuteGenerationBuildsImageEditRequestAndCompletesAsyncPoll(t *testing.T) {
	t.Parallel()

	provider := &stubImageProvider{
		name:  "azure_foundry",
		model: "FLUX.2-pro",
		generateFn: func(context.Context, imageprovider.GenerateRequest) (imageprovider.GenerateResult, error) {
			return imageprovider.GenerateResult{
				Job: &imageprovider.ProviderJob{JobID: "job-123"},
			}, nil
		},
		pollFn: func(context.Context, imageprovider.ProviderJob) (imageprovider.PollResult, error) {
			return imageprovider.PollResult{
				Status: imageprovider.PollStatusCompleted,
				Images: []imageprovider.OutputImage{{
					URL: "https://cdn.example.com/output.png",
				}},
			}, nil
		},
	}
	blobStore := &stubBlobStore{
		downloads: map[string][]byte{
			"sessions/s/image-generations/g/inputs/01-ref.png": testPNGData(),
		},
	}
	generationRepo := &stubGenerationReader{
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
			CreatedAt:           time.Now().UTC(),
		},
	}
	tx := &stubWriteTx{}
	resolverCalled := false
	service := &Service{
		blobs:    blobStore,
		provider: provider,
		outputResolver: stubHostnameResolver{lookupIPAddr: func(ctx context.Context, host string) ([]net.IPAddr, error) {
			resolverCalled = true
			if host != "cdn.example.com" {
				t.Fatalf("host = %q", host)
			}
			return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
		}},
		outputHTTPClient: stubHTTPDoer{do: func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != "https://cdn.example.com/output.png" {
				t.Fatalf("req.URL.String() = %q", req.URL.String())
			}
			return newHTTPResponse(http.StatusOK, "image/png", testPNGData()), nil
		}},
		beginWriteTxFn: func(context.Context) (writeTx, error) { return tx, nil },
		generationRead: generationRepo,
		assetRead: stubAssetReader{
			assets: []repository.ImageGenerationAsset{{
				ID:           "asset-input",
				GenerationID: generationRepo.generation.ID,
				Role:         "input",
				SortOrder:    0,
				BlobPath:     "sessions/s/image-generations/g/inputs/01-ref.png",
				MediaType:    "image/png",
				Filename:     "ref.png",
				SizeBytes:    int64(len(testPNGData())),
				Sha256:       "abc",
				CreatedAt:    time.Now().UTC(),
			}},
		},
	}

	if err := service.ExecuteGeneration(context.Background(), generationRepo.generation.ID); err != nil {
		t.Fatalf("first ExecuteGeneration() error = %v", err)
	}
	if len(provider.generateRequests) != 1 {
		t.Fatalf("len(provider.generateRequests) = %d, want 1", len(provider.generateRequests))
	}
	req := provider.generateRequests[0]
	if req.Mode != imageprovider.ModeImageEdit || len(req.InputImages) != 1 {
		t.Fatalf("provider generate request = %+v", req)
	}
	if !slices.Equal(req.InputImages[0], testPNGData()) {
		t.Fatalf("provider input image = %v, want %v", req.InputImages[0], testPNGData())
	}
	if len(generationRepo.updatedStates) != 2 {
		t.Fatalf("len(generationRepo.updatedStates) = %d, want 2", len(generationRepo.updatedStates))
	}
	if generationRepo.updatedStates[1].ProviderJobID != "job-123" || generationRepo.updatedStates[1].Status != "running" {
		t.Fatalf("job metadata state = %+v", generationRepo.updatedStates[1])
	}
	tx.lockGeneration = generationRepo.generation

	if err := service.ExecuteGeneration(context.Background(), generationRepo.generation.ID); err != nil {
		t.Fatalf("second ExecuteGeneration() error = %v", err)
	}
	if len(provider.pollJobs) != 1 || provider.pollJobs[0].JobID != "job-123" {
		t.Fatalf("provider.pollJobs = %+v", provider.pollJobs)
	}
	if len(tx.createdAssets) != 1 || tx.createdAssets[0].Role != "output" {
		t.Fatalf("tx.createdAssets = %+v", tx.createdAssets)
	}
	if len(tx.updatedStates) != 1 || tx.updatedStates[0].Status != "completed" {
		t.Fatalf("tx.updatedStates = %+v", tx.updatedStates)
	}
	if tx.updatedStates[0].ProviderJobID != "job-123" {
		t.Fatalf("completed state provider job id = %q", tx.updatedStates[0].ProviderJobID)
	}
	if !resolverCalled {
		t.Fatal("expected output hostname resolver to be called")
	}
}

func TestExecuteGenerationSkipsProviderWhenPendingClaimIsLost(t *testing.T) {
	t.Parallel()

	provider := &stubImageProvider{
		generateFn: func(context.Context, imageprovider.GenerateRequest) (imageprovider.GenerateResult, error) {
			t.Fatal("provider should not be called when pending claim is lost")
			return imageprovider.GenerateResult{}, nil
		},
	}
	generationRepo := &stubGenerationReader{
		generation: repository.ImageGeneration{
			ID:                  "22222222-2222-2222-2222-222222222222",
			SessionID:           "11111111-1111-1111-1111-111111111111",
			Mode:                "text_to_image",
			Prompt:              "draw a lighthouse",
			ResolutionKey:       "1024x1024",
			Width:               1024,
			Height:              1024,
			RequestedImageCount: 1,
			Status:              "pending",
			CreatedAt:           time.Now().UTC(),
		},
		claimPendingBlocked: true,
	}
	service := &Service{
		blobs:            &stubBlobStore{},
		provider:         provider,
		outputHTTPClient: &http.Client{Timeout: time.Second},
		beginWriteTxFn:   func(context.Context) (writeTx, error) { return &stubWriteTx{}, nil },
		generationRead:   generationRepo,
		assetRead:        stubAssetReader{err: errors.New("should not list input assets after claim loss")},
	}

	if err := service.ExecuteGeneration(context.Background(), generationRepo.generation.ID); err != nil {
		t.Fatalf("ExecuteGeneration() error = %v", err)
	}
	if generationRepo.claimPendingCalls != 1 {
		t.Fatalf("claimPendingCalls = %d, want 1", generationRepo.claimPendingCalls)
	}
	if len(provider.generateRequests) != 0 {
		t.Fatalf("provider.generateRequests = %+v", provider.generateRequests)
	}
	if len(generationRepo.updatedStates) != 0 {
		t.Fatalf("generationRepo.updatedStates = %+v", generationRepo.updatedStates)
	}
}

func TestExecuteRunningGenerationWithMissingJobIDIsNoop(t *testing.T) {
	t.Parallel()

	startedAt := time.Now().UTC()
	provider := &stubImageProvider{
		pollFn: func(context.Context, imageprovider.ProviderJob) (imageprovider.PollResult, error) {
			t.Fatal("provider should not be polled while provider job id is not yet persisted")
			return imageprovider.PollResult{}, nil
		},
	}
	generationRepo := &stubGenerationReader{
		generation: repository.ImageGeneration{
			ID:        "22222222-2222-2222-2222-222222222222",
			SessionID: "11111111-1111-1111-1111-111111111111",
			Status:    "running",
			StartedAt: &startedAt,
		},
	}
	service := &Service{
		blobs:            &stubBlobStore{},
		provider:         provider,
		outputHTTPClient: &http.Client{Timeout: time.Second},
		beginWriteTxFn:   func(context.Context) (writeTx, error) { return &stubWriteTx{}, nil },
		generationRead:   generationRepo,
		assetRead:        stubAssetReader{},
	}

	if err := service.ExecuteGeneration(context.Background(), generationRepo.generation.ID); err != nil {
		t.Fatalf("ExecuteGeneration() error = %v", err)
	}
	if len(generationRepo.updatedStates) != 0 {
		t.Fatalf("generationRepo.updatedStates = %+v", generationRepo.updatedStates)
	}
}

func TestExecuteRunningGenerationWithStaleMissingJobIDFails(t *testing.T) {
	t.Parallel()

	startedAt := time.Now().UTC().Add(-providerJobIDPersistGracePeriod - time.Second)
	provider := &stubImageProvider{
		pollFn: func(context.Context, imageprovider.ProviderJob) (imageprovider.PollResult, error) {
			t.Fatal("provider should not be polled without a provider job id")
			return imageprovider.PollResult{}, nil
		},
	}
	generationRepo := &stubGenerationReader{
		generation: repository.ImageGeneration{
			ID:        "22222222-2222-2222-2222-222222222222",
			SessionID: "11111111-1111-1111-1111-111111111111",
			Status:    "running",
			StartedAt: &startedAt,
		},
	}
	service := &Service{
		blobs:            &stubBlobStore{},
		provider:         provider,
		outputHTTPClient: &http.Client{Timeout: time.Second},
		beginWriteTxFn:   func(context.Context) (writeTx, error) { return &stubWriteTx{}, nil },
		generationRead:   generationRepo,
		assetRead:        stubAssetReader{},
	}

	err := service.ExecuteGeneration(context.Background(), generationRepo.generation.ID)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "provider job id was not persisted") {
		t.Fatalf("err = %v", err)
	}
	if len(generationRepo.updatedStates) != 1 {
		t.Fatalf("generationRepo.updatedStates = %+v", generationRepo.updatedStates)
	}
	failedState := generationRepo.updatedStates[0]
	if failedState.Status != "failed" || failedState.ErrorCode != "missing_provider_job_id" {
		t.Fatalf("failedState = %+v", failedState)
	}
}

func TestExecuteGenerationMarksFailureWhenJobMetadataPersistenceFails(t *testing.T) {
	t.Parallel()

	provider := &stubImageProvider{
		name:  "azure_foundry",
		model: "FLUX.2-pro",
		generateFn: func(context.Context, imageprovider.GenerateRequest) (imageprovider.GenerateResult, error) {
			return imageprovider.GenerateResult{
				Job: &imageprovider.ProviderJob{JobID: "job-123"},
			}, nil
		},
	}
	generationRepo := &stubGenerationReader{
		generation: repository.ImageGeneration{
			ID:                  "22222222-2222-2222-2222-222222222222",
			SessionID:           "11111111-1111-1111-1111-111111111111",
			Mode:                "text_to_image",
			Prompt:              "draw a lighthouse",
			ResolutionKey:       "1024x1024",
			Width:               1024,
			Height:              1024,
			RequestedImageCount: 1,
			Status:              "pending",
			CreatedAt:           time.Now().UTC(),
		},
		updateStateErrors: []error{errors.New("update failed")},
	}
	service := &Service{
		blobs:            &stubBlobStore{},
		provider:         provider,
		outputHTTPClient: &http.Client{Timeout: time.Second},
		beginWriteTxFn:   func(context.Context) (writeTx, error) { return &stubWriteTx{}, nil },
		generationRead:   generationRepo,
		assetRead:        stubAssetReader{},
	}

	err := service.ExecuteGeneration(context.Background(), generationRepo.generation.ID)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "persist image generation job metadata") {
		t.Fatalf("err = %v", err)
	}
	if len(generationRepo.updatedStates) != 2 {
		t.Fatalf("len(generationRepo.updatedStates) = %d, want 2", len(generationRepo.updatedStates))
	}
	failedState := generationRepo.updatedStates[1]
	if failedState.Status != "failed" || failedState.ErrorCode != "persist_generation_state_failed" {
		t.Fatalf("failedState = %+v", failedState)
	}
	if failedState.ProviderJobID != "job-123" {
		t.Fatalf("failedState.ProviderJobID = %q", failedState.ProviderJobID)
	}
}

func TestExecuteGenerationMarksFailureWhenProviderRequestFails(t *testing.T) {
	t.Parallel()

	provider := &stubImageProvider{
		generateFn: func(context.Context, imageprovider.GenerateRequest) (imageprovider.GenerateResult, error) {
			return imageprovider.GenerateResult{}, imageprovider.APIError{
				StatusCode: http.StatusServiceUnavailable,
				Message:    "upstream unavailable",
			}
		},
	}
	generationRepo := &stubGenerationReader{
		generation: repository.ImageGeneration{
			ID:                  "22222222-2222-2222-2222-222222222222",
			SessionID:           "11111111-1111-1111-1111-111111111111",
			Mode:                "text_to_image",
			Prompt:              "draw a lighthouse",
			ResolutionKey:       "1024x1024",
			Width:               1024,
			Height:              1024,
			RequestedImageCount: 1,
			Status:              "pending",
			CreatedAt:           time.Now().UTC(),
		},
	}
	service := &Service{
		blobs:            &stubBlobStore{},
		provider:         provider,
		outputHTTPClient: &http.Client{Timeout: time.Second},
		beginWriteTxFn:   func(context.Context) (writeTx, error) { return &stubWriteTx{}, nil },
		generationRead:   generationRepo,
		assetRead:        stubAssetReader{},
	}

	err := service.ExecuteGeneration(context.Background(), generationRepo.generation.ID)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if len(generationRepo.updatedStates) != 2 {
		t.Fatalf("len(generationRepo.updatedStates) = %d, want 2", len(generationRepo.updatedStates))
	}
	failedState := generationRepo.updatedStates[1]
	if failedState.Status != "failed" || failedState.ErrorCode != "provider_http_503" {
		t.Fatalf("failedState = %+v", failedState)
	}
	if !strings.Contains(failedState.ErrorMessage, "upstream unavailable") {
		t.Fatalf("failedState.ErrorMessage = %q", failedState.ErrorMessage)
	}
}

func TestExecuteGenerationMarksFailureWhenProviderRequestFailsWithPointerAPIError(t *testing.T) {
	t.Parallel()

	provider := stubImageProviderNoMetadata{
		generateFn: func(context.Context, imageprovider.GenerateRequest) (imageprovider.GenerateResult, error) {
			return imageprovider.GenerateResult{}, &imageprovider.APIError{
				StatusCode: http.StatusBadGateway,
				Message:    "gateway failed",
			}
		},
	}
	generationRepo := &stubGenerationReader{
		generation: repository.ImageGeneration{
			ID:            "22222222-2222-2222-2222-222222222222",
			SessionID:     "11111111-1111-1111-1111-111111111111",
			Mode:          "text_to_image",
			Prompt:        "draw a lighthouse",
			ResolutionKey: "1024x1024",
			Width:         1024,
			Height:        1024,
			Status:        "pending",
			CreatedAt:     time.Now().UTC(),
		},
	}
	service := &Service{
		blobs:            &stubBlobStore{},
		provider:         provider,
		outputHTTPClient: &http.Client{Timeout: time.Second},
		beginWriteTxFn:   func(context.Context) (writeTx, error) { return &stubWriteTx{}, nil },
		generationRead:   generationRepo,
		assetRead:        stubAssetReader{},
	}

	err := service.ExecuteGeneration(context.Background(), generationRepo.generation.ID)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if len(generationRepo.updatedStates) != 2 {
		t.Fatalf("len(generationRepo.updatedStates) = %d, want 2", len(generationRepo.updatedStates))
	}
	failedState := generationRepo.updatedStates[1]
	if failedState.ErrorCode != "provider_http_502" {
		t.Fatalf("failedState = %+v", failedState)
	}
}

func TestExecuteGenerationPreservesProviderMetadataOnCompletedPollWithoutProviderMetadata(t *testing.T) {
	t.Parallel()

	service := &Service{
		blobs: &stubBlobStore{},
		provider: stubImageProviderNoMetadata{
			pollFn: func(context.Context, imageprovider.ProviderJob) (imageprovider.PollResult, error) {
				return imageprovider.PollResult{
					Status: imageprovider.PollStatusCompleted,
					Images: []imageprovider.OutputImage{{
						Data:      slices.Clone(testPNGData()),
						MediaType: "image/png",
					}},
				}, nil
			},
		},
		outputHTTPClient: &http.Client{Timeout: time.Second},
		beginWriteTxFn:   func(context.Context) (writeTx, error) { return &stubWriteTx{}, nil },
		generationRead:   &stubGenerationReader{},
		assetRead:        stubAssetReader{},
	}

	generation := repository.ImageGeneration{
		ID:            "22222222-2222-2222-2222-222222222222",
		SessionID:     "11111111-1111-1111-1111-111111111111",
		Status:        "running",
		ProviderJobID: "job-123",
		ProviderName:  "azure_foundry",
		ProviderModel: "FLUX.2-pro",
	}
	tx := &stubWriteTx{lockGeneration: generation}
	service.beginWriteTxFn = func(context.Context) (writeTx, error) { return tx, nil }

	if err := service.completeGeneration(context.Background(), generation, []imageprovider.OutputImage{{
		Data:      slices.Clone(testPNGData()),
		MediaType: "image/png",
	}}); err != nil {
		t.Fatalf("completeGeneration() error = %v", err)
	}
	if len(tx.updatedStates) != 1 {
		t.Fatalf("len(tx.updatedStates) = %d, want 1", len(tx.updatedStates))
	}
	if tx.updatedStates[0].ProviderName != "azure_foundry" || tx.updatedStates[0].ProviderModel != "FLUX.2-pro" {
		t.Fatalf("completed state metadata = %+v", tx.updatedStates[0])
	}
}

func TestCompleteGenerationSkipsOutputWhenAlreadyTerminal(t *testing.T) {
	t.Parallel()

	completedAt := time.Now().UTC()
	tx := &stubWriteTx{
		lockGeneration: repository.ImageGeneration{
			ID:          "22222222-2222-2222-2222-222222222222",
			SessionID:   "11111111-1111-1111-1111-111111111111",
			Status:      "completed",
			CompletedAt: &completedAt,
		},
	}
	blobStore := &stubBlobStore{}
	service := &Service{
		blobs:            blobStore,
		outputHTTPClient: &http.Client{Timeout: time.Second},
		beginWriteTxFn:   func(context.Context) (writeTx, error) { return tx, nil },
		generationRead:   &stubGenerationReader{},
		assetRead:        stubAssetReader{},
	}

	err := service.completeGeneration(context.Background(), repository.ImageGeneration{
		ID:        "22222222-2222-2222-2222-222222222222",
		SessionID: "11111111-1111-1111-1111-111111111111",
		Status:    "running",
	}, []imageprovider.OutputImage{{
		Data:      slices.Clone(testPNGData()),
		MediaType: "image/png",
	}})
	if err != nil {
		t.Fatalf("completeGeneration() error = %v", err)
	}
	if tx.lockCalls != 1 {
		t.Fatalf("lockCalls = %d, want 1", tx.lockCalls)
	}
	if !tx.rolledBack {
		t.Fatal("expected transaction rollback")
	}
	if len(blobStore.uploads) != 0 || len(tx.createdAssets) != 0 || len(tx.updatedStates) != 0 {
		t.Fatalf("unexpected persistence: uploads=%+v createdAssets=%+v updatedStates=%+v", blobStore.uploads, tx.createdAssets, tx.updatedStates)
	}
}

func TestPersistGenerationFailurePreservesStoredProviderMetadata(t *testing.T) {
	t.Parallel()

	generationRepo := &stubGenerationReader{}
	service := &Service{
		provider:       stubImageProviderNoMetadata{},
		generationRead: generationRepo,
	}

	err := service.persistGenerationFailure(context.Background(), repository.ImageGeneration{
		ID:            "22222222-2222-2222-2222-222222222222",
		ProviderJobID: "job-123",
		ProviderName:  "azure_foundry",
		ProviderModel: "FLUX.2-pro",
	}, "provider_job_failed", "provider job failed")
	if err != nil {
		t.Fatalf("persistGenerationFailure() error = %v", err)
	}
	if len(generationRepo.updatedStates) != 1 {
		t.Fatalf("len(generationRepo.updatedStates) = %d, want 1", len(generationRepo.updatedStates))
	}
	if generationRepo.updatedStates[0].ProviderName != "azure_foundry" || generationRepo.updatedStates[0].ProviderModel != "FLUX.2-pro" {
		t.Fatalf("failed state metadata = %+v", generationRepo.updatedStates[0])
	}
}

func TestLoadInputImagesRejectsOversizedDownloadedInput(t *testing.T) {
	t.Parallel()

	service := &Service{
		blobs: &stubBlobStore{
			downloads: map[string][]byte{
				"blob-path": testPNGData(),
			},
		},
		maxReferenceImageBytes: 8,
	}

	_, err := service.loadInputImages(context.Background(), []repository.ImageGenerationAsset{{
		ID:       "asset-input",
		Role:     "input",
		BlobPath: "blob-path",
		Filename: "ref.png",
	}})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `download input asset "ref.png": blob "blob-path" exceeds 8 bytes`) {
		t.Fatalf("err = %v", err)
	}
}

func TestExecuteGenerationCleansUpOutputBlobWhenPersistenceFails(t *testing.T) {
	t.Parallel()

	provider := &stubImageProvider{
		generateFn: func(context.Context, imageprovider.GenerateRequest) (imageprovider.GenerateResult, error) {
			return imageprovider.GenerateResult{
				Images: []imageprovider.OutputImage{{
					Data:      slices.Clone(testPNGData()),
					MediaType: "image/png",
				}},
			}, nil
		},
	}
	blobStore := &stubBlobStore{}
	generationRepo := &stubGenerationReader{
		generation: repository.ImageGeneration{
			ID:                  "22222222-2222-2222-2222-222222222222",
			SessionID:           "11111111-1111-1111-1111-111111111111",
			Mode:                "text_to_image",
			Prompt:              "draw a lighthouse",
			ResolutionKey:       "1024x1024",
			Width:               1024,
			Height:              1024,
			RequestedImageCount: 1,
			Status:              "pending",
			CreatedAt:           time.Now().UTC(),
		},
	}
	tx := &stubWriteTx{
		createAssetErr: errors.New("insert failed"),
		lockGeneration: repository.ImageGeneration{
			ID:                  generationRepo.generation.ID,
			SessionID:           generationRepo.generation.SessionID,
			Mode:                generationRepo.generation.Mode,
			Prompt:              generationRepo.generation.Prompt,
			ResolutionKey:       generationRepo.generation.ResolutionKey,
			Width:               generationRepo.generation.Width,
			Height:              generationRepo.generation.Height,
			RequestedImageCount: generationRepo.generation.RequestedImageCount,
			Status:              "running",
			CreatedAt:           generationRepo.generation.CreatedAt,
		},
	}
	service := &Service{
		blobs:            blobStore,
		provider:         provider,
		outputHTTPClient: &http.Client{Timeout: time.Second},
		beginWriteTxFn:   func(context.Context) (writeTx, error) { return tx, nil },
		generationRead:   generationRepo,
		assetRead:        stubAssetReader{},
	}

	err := service.ExecuteGeneration(context.Background(), generationRepo.generation.ID)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if len(blobStore.uploads) != 1 {
		t.Fatalf("len(blobStore.uploads) = %d, want 1", len(blobStore.uploads))
	}
	if len(blobStore.deletedPaths) != 1 || blobStore.deletedPaths[0] != blobStore.uploads[0].blobPath {
		t.Fatalf("blob cleanup mismatch: uploads=%+v deleted=%+v", blobStore.uploads, blobStore.deletedPaths)
	}
	if !tx.rolledBack {
		t.Fatal("expected transaction rollback")
	}
	if len(generationRepo.updatedStates) != 2 {
		t.Fatalf("len(generationRepo.updatedStates) = %d, want 2", len(generationRepo.updatedStates))
	}
	failedState := generationRepo.updatedStates[1]
	if failedState.Status != "failed" || failedState.ErrorCode != "persist_output_failed" {
		t.Fatalf("failedState = %+v", failedState)
	}
}

func TestResolveOutputImageDataRejectsOversizedInlineData(t *testing.T) {
	t.Parallel()

	service := &Service{}

	_, _, err := service.resolveOutputImageData(context.Background(), imageprovider.OutputImage{
		Data: make([]byte, int(maxOutputImageBytes)+1),
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "inline output image exceeds") {
		t.Fatalf("err = %v", err)
	}
}

func TestResolveOutputImageDataDoesNotFollowRedirects(t *testing.T) {
	t.Parallel()

	service := &Service{
		outputResolver: stubHostnameResolver{lookupIPAddr: func(ctx context.Context, host string) ([]net.IPAddr, error) {
			if host != "cdn.example.com" {
				t.Fatalf("host = %q", host)
			}
			return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
		}},
		outputHTTPClient: stubHTTPDoer{do: func(req *http.Request) (*http.Response, error) {
			return newHTTPResponse(http.StatusFound, "", nil), nil
		}},
	}

	_, _, err := service.resolveOutputImageData(context.Background(), imageprovider.OutputImage{
		URL: "https://cdn.example.com/source.png",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unexpected status 302") {
		t.Fatalf("err = %v", err)
	}
}

func TestValidatedOutputDialAddressUsesResolvedPublicIP(t *testing.T) {
	t.Parallel()

	address, err := validatedOutputDialAddress(context.Background(), stubHostnameResolver{
		lookupIPAddr: func(ctx context.Context, host string) ([]net.IPAddr, error) {
			if host != "cdn.example.com" {
				t.Fatalf("host = %q", host)
			}
			return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
		},
	}, "cdn.example.com:443")
	if err != nil {
		t.Fatalf("validatedOutputDialAddress() error = %v", err)
	}
	if address != "93.184.216.34:443" {
		t.Fatalf("address = %q", address)
	}
}

func TestValidatedOutputDialAddressRejectsResolvedPrivateIP(t *testing.T) {
	t.Parallel()

	_, err := validatedOutputDialAddress(context.Background(), stubHostnameResolver{
		lookupIPAddr: func(ctx context.Context, host string) ([]net.IPAddr, error) {
			return []net.IPAddr{{IP: net.ParseIP("10.0.0.4")}}, nil
		},
	}, "cdn.example.com:443")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "provider output URL host is not allowed") {
		t.Fatalf("err = %v", err)
	}
}

func TestOutputHTTPClientDisablesProxy(t *testing.T) {
	t.Parallel()

	client := newOutputHTTPClient(time.Second, stubHostnameResolver{
		lookupIPAddr: func(ctx context.Context, host string) ([]net.IPAddr, error) {
			return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
		},
	})
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("client.Transport = %T, want *http.Transport", client.Transport)
	}
	req, err := http.NewRequest(http.MethodGet, "https://cdn.example.com/output.png", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	proxyURL, err := transport.Proxy(req)
	if err != nil {
		t.Fatalf("transport.Proxy() error = %v", err)
	}
	if proxyURL != nil {
		t.Fatalf("proxyURL = %v, want nil", proxyURL)
	}
}

type stubBlobStore struct {
	uploads      []stubBlobUpload
	deletedPaths []string
	downloads    map[string][]byte
}

type stubBlobUpload struct {
	blobPath    string
	contentType string
}

type stubHTTPDoer struct {
	do func(req *http.Request) (*http.Response, error)
}

type stubHostnameResolver struct {
	lookupIPAddr func(ctx context.Context, host string) ([]net.IPAddr, error)
}

func (s stubHTTPDoer) Do(req *http.Request) (*http.Response, error) {
	return s.do(req)
}

func (s stubHostnameResolver) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	return s.lookupIPAddr(ctx, host)
}

func newHTTPResponse(statusCode int, contentType string, data []byte) *http.Response {
	header := make(http.Header)
	if contentType != "" {
		header.Set("Content-Type", contentType)
	}

	return &http.Response{
		StatusCode: statusCode,
		Header:     header,
		Body:       io.NopCloser(bytes.NewReader(data)),
	}
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

func (s *stubBlobStore) Download(ctx context.Context, blobPath string) ([]byte, error) {
	data, ok := s.downloads[blobPath]
	if !ok {
		return nil, errors.New("blob not found")
	}
	return slices.Clone(data), nil
}

func (s *stubBlobStore) DownloadWithinLimit(ctx context.Context, blobPath string, maxBytes int64) ([]byte, error) {
	data, err := s.Download(ctx, blobPath)
	if err != nil {
		return nil, err
	}
	if maxBytes > 0 && int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("blob %q exceeds %d bytes", blobPath, maxBytes)
	}
	return data, nil
}

type stubWriteTx struct {
	session             repository.Session
	getSessionErr       error
	generation          repository.ImageGeneration
	lockGeneration      repository.ImageGeneration
	lockGenerationErr   error
	lockCalls           int
	createGenerationErr error
	createAssetErr      error
	updateStateErr      error
	createdGenerations  []repository.CreateImageGenerationParams
	createdAssets       []repository.CreateImageGenerationAssetParams
	updatedStates       []repository.UpdateImageGenerationStateParams
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

func (s *stubWriteTx) LockGenerationForUpdate(ctx context.Context, generationID string) (repository.ImageGeneration, error) {
	s.lockCalls++
	if s.lockGenerationErr != nil {
		return repository.ImageGeneration{}, s.lockGenerationErr
	}
	return s.lockGeneration, nil
}

func (s *stubWriteTx) UpdateGenerationState(ctx context.Context, params repository.UpdateImageGenerationStateParams) error {
	if s.updateStateErr != nil {
		return s.updateStateErr
	}
	s.updatedStates = append(s.updatedStates, params)
	return nil
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
	generation          repository.ImageGeneration
	err                 error
	claimPendingErr     error
	claimPendingBlocked bool
	claimPendingCalls   int
	updateStateErrors   []error
	updatedStates       []repository.UpdateImageGenerationStateParams
}

func (s *stubGenerationReader) GetByID(ctx context.Context, generationID string) (repository.ImageGeneration, error) {
	if s.err != nil {
		return repository.ImageGeneration{}, s.err
	}
	return s.generation, nil
}

func (s *stubGenerationReader) GetByIDAndSession(ctx context.Context, generationID string, sessionID string) (repository.ImageGeneration, error) {
	if s.err != nil {
		return repository.ImageGeneration{}, s.err
	}
	return s.generation, nil
}

func (s *stubGenerationReader) ClaimPending(ctx context.Context, params repository.ClaimPendingImageGenerationParams) (bool, error) {
	s.claimPendingCalls++
	if s.claimPendingErr != nil {
		return false, s.claimPendingErr
	}
	if s.claimPendingBlocked {
		return false, nil
	}

	state := repository.UpdateImageGenerationStateParams{
		ID:            params.ID,
		ProviderName:  params.ProviderName,
		ProviderModel: params.ProviderModel,
		Status:        string(StatusRunning),
	}
	s.updatedStates = append(s.updatedStates, state)
	s.generation.ProviderName = params.ProviderName
	s.generation.ProviderModel = params.ProviderModel
	s.generation.ProviderJobID = ""
	s.generation.Status = string(StatusRunning)
	s.generation.ErrorCode = ""
	s.generation.ErrorMessage = ""
	startedAt := time.Now().UTC()
	s.generation.StartedAt = &startedAt
	s.generation.CompletedAt = nil

	return true, nil
}

func (s *stubGenerationReader) UpdateState(ctx context.Context, params repository.UpdateImageGenerationStateParams) error {
	if len(s.updateStateErrors) > 0 {
		err := s.updateStateErrors[0]
		s.updateStateErrors = s.updateStateErrors[1:]
		if err != nil {
			return err
		}
	}
	s.updatedStates = append(s.updatedStates, params)
	s.generation.ProviderName = params.ProviderName
	s.generation.ProviderModel = params.ProviderModel
	s.generation.ProviderJobID = params.ProviderJobID
	s.generation.Status = params.Status
	s.generation.ErrorCode = params.ErrorCode
	s.generation.ErrorMessage = params.ErrorMessage
	s.generation.CompletedAt = params.CompletedAt
	return nil
}

type stubAssetReader struct {
	assets []repository.ImageGenerationAsset
	err    error
}

func (s stubAssetReader) ListByGeneration(ctx context.Context, generationID string) ([]repository.ImageGenerationAsset, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.assets, nil
}

func (s stubAssetReader) ListByGenerationAndSession(ctx context.Context, generationID string, sessionID string) ([]repository.ImageGenerationAsset, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.assets, nil
}

type stubImageProvider struct {
	name             string
	model            string
	generateFn       func(context.Context, imageprovider.GenerateRequest) (imageprovider.GenerateResult, error)
	pollFn           func(context.Context, imageprovider.ProviderJob) (imageprovider.PollResult, error)
	generateRequests []imageprovider.GenerateRequest
	pollJobs         []imageprovider.ProviderJob
}

func (s *stubImageProvider) Generate(ctx context.Context, req imageprovider.GenerateRequest) (imageprovider.GenerateResult, error) {
	s.generateRequests = append(s.generateRequests, req)
	if s.generateFn != nil {
		return s.generateFn(ctx, req)
	}
	return imageprovider.GenerateResult{}, nil
}

func (s *stubImageProvider) Poll(ctx context.Context, job imageprovider.ProviderJob) (imageprovider.PollResult, error) {
	s.pollJobs = append(s.pollJobs, job)
	if s.pollFn != nil {
		return s.pollFn(ctx, job)
	}
	return imageprovider.PollResult{}, nil
}

func (s *stubImageProvider) ProviderName() string {
	return s.name
}

func (s *stubImageProvider) ProviderModel() string {
	return s.model
}

type stubImageProviderNoMetadata struct {
	generateFn func(context.Context, imageprovider.GenerateRequest) (imageprovider.GenerateResult, error)
	pollFn     func(context.Context, imageprovider.ProviderJob) (imageprovider.PollResult, error)
}

func (s stubImageProviderNoMetadata) Generate(ctx context.Context, req imageprovider.GenerateRequest) (imageprovider.GenerateResult, error) {
	if s.generateFn != nil {
		return s.generateFn(ctx, req)
	}
	return imageprovider.GenerateResult{}, nil
}

func (s stubImageProviderNoMetadata) Poll(ctx context.Context, job imageprovider.ProviderJob) (imageprovider.PollResult, error) {
	if s.pollFn != nil {
		return s.pollFn(ctx, job)
	}
	return imageprovider.PollResult{}, nil
}

func testPNGData() []byte {
	return []byte{
		0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n',
		0x00, 0x00, 0x00, 0x0d, 'I', 'H', 'D', 'R',
		0x00, 0x00, 0x00, 0x01,
		0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00,
	}
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) {
	return 0, errors.New("read failed")
}
