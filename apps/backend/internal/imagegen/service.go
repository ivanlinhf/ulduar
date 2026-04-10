package imagegen

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/ivanlin/ulduar/apps/backend/internal/filenames"
	"github.com/ivanlin/ulduar/apps/backend/internal/imageprovider"
	"github.com/ivanlin/ulduar/apps/backend/internal/repository"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	supportedInputMediaTypes = map[string]struct{}{
		"image/jpeg": {},
		"image/png":  {},
		"image/webp": {},
	}
	supportedOutputMediaTypes = map[string]struct{}{
		"image/jpeg": {},
		"image/png":  {},
		"image/webp": {},
	}
)

const (
	defaultOutputFormat                   = "png"
	defaultOutputFilenamePrefix           = "output"
	defaultOutputDownloadTimeout          = 30 * time.Second
	maxOutputImageBytes             int64 = 32 << 20
	providerJobIDPersistGracePeriod       = 10 * time.Minute
)

var blockedOutputIPPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("169.254.0.0/16"),
	netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.168.0.0/16"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("224.0.0.0/4"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("::/128"),
	netip.MustParsePrefix("::1/128"),
	netip.MustParsePrefix("100::/64"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("fc00::/7"),
	netip.MustParsePrefix("fe80::/10"),
	netip.MustParsePrefix("ff00::/8"),
}

type BlobStore interface {
	Upload(ctx context.Context, blobPath string, data []byte, contentType string) error
	Delete(ctx context.Context, blobPath string) error
	DownloadWithinLimit(ctx context.Context, blobPath string, maxBytes int64) ([]byte, error)
}

type ValidationError struct {
	StatusCode int
	Message    string
}

func (e ValidationError) Error() string {
	return e.Message
}

type writeTx interface {
	GetSession(ctx context.Context, sessionID string) (repository.Session, error)
	CreateGeneration(ctx context.Context, params repository.CreateImageGenerationParams) (repository.ImageGeneration, error)
	CreateGenerationAsset(ctx context.Context, params repository.CreateImageGenerationAssetParams) (repository.ImageGenerationAsset, error)
	LockGenerationForUpdate(ctx context.Context, generationID string) (repository.ImageGeneration, error)
	UpdateGenerationState(ctx context.Context, params repository.UpdateImageGenerationStateParams) error
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

type generationReader interface {
	GetByID(ctx context.Context, generationID string) (repository.ImageGeneration, error)
	GetByIDAndSession(ctx context.Context, generationID string, sessionID string) (repository.ImageGeneration, error)
	ClaimPending(ctx context.Context, params repository.ClaimPendingImageGenerationParams) (bool, error)
	UpdateState(ctx context.Context, params repository.UpdateImageGenerationStateParams) error
}

type assetReader interface {
	ListByGeneration(ctx context.Context, generationID string) ([]repository.ImageGenerationAsset, error)
	ListByGenerationAndSession(ctx context.Context, generationID string, sessionID string) ([]repository.ImageGenerationAsset, error)
}

type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type hostnameResolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}

type Service struct {
	blobs                  BlobStore
	provider               imageprovider.ImageProvider
	outputHTTPClient       httpDoer
	outputResolver         hostnameResolver
	maxReferenceImageBytes int64
	beginWriteTxFn         func(ctx context.Context) (writeTx, error)
	generationRead         generationReader
	assetRead              assetReader
}

type ServiceOptions struct {
	MaxReferenceImageBytes int64
	Provider               imageprovider.ImageProvider
	OutputDownloadTimeout  time.Duration
}

// NewService accepts zero or one ServiceOptions value.
// When options are omitted, defaults are applied.
func NewService(db *pgxpool.Pool, blobs BlobStore, options ...ServiceOptions) *Service {
	if len(options) > 1 {
		panic("imagegen.NewService accepts at most one ServiceOptions value")
	}

	resolvedOptions := ServiceOptions{}
	if len(options) > 0 {
		resolvedOptions = options[0]
	}

	service := &Service{
		blobs:                  blobs,
		provider:               resolvedOptions.Provider,
		maxReferenceImageBytes: resolvedOptions.MaxReferenceImageBytes,
	}
	if service.maxReferenceImageBytes <= 0 {
		service.maxReferenceImageBytes = DefaultMaxReferenceImageBytes
	}
	outputDownloadTimeout := resolvedOptions.OutputDownloadTimeout
	if outputDownloadTimeout <= 0 {
		outputDownloadTimeout = defaultOutputDownloadTimeout
	}
	service.outputResolver = net.DefaultResolver
	service.outputHTTPClient = newOutputHTTPClient(outputDownloadTimeout, service.outputResolver)
	if db != nil {
		service.beginWriteTxFn = func(ctx context.Context) (writeTx, error) {
			tx, err := db.BeginTx(ctx, pgx.TxOptions{})
			if err != nil {
				return nil, fmt.Errorf("begin transaction: %w", err)
			}

			return repositoryWriteTx{tx: tx}, nil
		}
		service.generationRead = repository.NewImageGenerationRepository(db)
		service.assetRead = repository.NewImageGenerationAssetRepository(db)
	}

	return service
}

func (s *Service) CreatePendingGeneration(ctx context.Context, params CreateGenerationParams) (GenerationView, error) {
	if err := validateUUID(params.SessionID, "sessionId"); err != nil {
		return GenerationView{}, err
	}

	maxReferenceImageBytes := s.maxReferenceImageBytes
	if maxReferenceImageBytes <= 0 {
		maxReferenceImageBytes = DefaultMaxReferenceImageBytes
	}

	resolution, preparedAssets, prompt, err := validateCreateGenerationParams(params, maxReferenceImageBytes)
	if err != nil {
		return GenerationView{}, err
	}

	if s.beginWriteTxFn == nil {
		return GenerationView{}, fmt.Errorf("image generation service is not configured")
	}

	tx, err := s.beginWriteTxFn(ctx)
	if err != nil {
		return GenerationView{}, err
	}

	committed := false
	uploadedBlobPaths := make([]string, 0, len(preparedAssets))
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
			s.cleanupBlobs(uploadedBlobPaths)
		}
	}()

	if _, err := tx.GetSession(ctx, params.SessionID); err != nil {
		return GenerationView{}, mapRepositoryError(err, "session not found")
	}

	generationRecord, err := tx.CreateGeneration(ctx, repository.CreateImageGenerationParams{
		SessionID:           params.SessionID,
		Mode:                string(params.Mode),
		Prompt:              prompt,
		ResolutionKey:       resolution.Key,
		Width:               resolution.Width,
		Height:              resolution.Height,
		RequestedImageCount: OutputImageCountV1,
		ProviderName:        "",
		ProviderModel:       "",
		Status:              string(StatusPending),
	})
	if err != nil {
		return GenerationView{}, fmt.Errorf("create image generation: %w", err)
	}

	if len(preparedAssets) > 0 && s.blobs == nil {
		return GenerationView{}, fmt.Errorf("blob store is not configured")
	}

	assetRecords := make([]repository.ImageGenerationAsset, 0, len(preparedAssets))
	for index, asset := range preparedAssets {
		blobPath := buildInputBlobPath(params.SessionID, generationRecord.ID, index, asset)
		if err := s.blobs.Upload(ctx, blobPath, asset.Data, asset.MediaType); err != nil {
			return GenerationView{}, fmt.Errorf("store reference image %q: %w", asset.Filename, err)
		}
		uploadedBlobPaths = append(uploadedBlobPaths, blobPath)

		assetRecord, err := tx.CreateGenerationAsset(ctx, repository.CreateImageGenerationAssetParams{
			GenerationID: generationRecord.ID,
			Role:         string(AssetRoleInput),
			SortOrder:    int64(index),
			BlobPath:     blobPath,
			MediaType:    asset.MediaType,
			Filename:     asset.Filename,
			SizeBytes:    asset.SizeBytes,
			Sha256:       asset.SHA256,
			Width:        asset.Width,
			Height:       asset.Height,
		})
		if err != nil {
			return GenerationView{}, fmt.Errorf("persist reference image %q: %w", asset.Filename, err)
		}

		assetRecords = append(assetRecords, assetRecord)
	}

	if err := tx.Commit(ctx); err != nil {
		return GenerationView{}, fmt.Errorf("commit transaction: %w", err)
	}
	committed = true

	return GenerationView{
		Generation: mapGeneration(generationRecord),
		Assets:     mapAssets(assetRecords),
	}, nil
}

func (s *Service) GetGeneration(ctx context.Context, sessionID, generationID string) (GenerationView, error) {
	if err := validateUUID(sessionID, "sessionId"); err != nil {
		return GenerationView{}, err
	}
	if err := validateUUID(generationID, "generationId"); err != nil {
		return GenerationView{}, err
	}
	if s.generationRead == nil || s.assetRead == nil {
		return GenerationView{}, fmt.Errorf("image generation service is not configured")
	}

	generationRecord, err := s.generationRead.GetByIDAndSession(ctx, generationID, sessionID)
	if err != nil {
		return GenerationView{}, mapRepositoryError(err, "image generation not found")
	}

	assetRecords, err := s.assetRead.ListByGenerationAndSession(ctx, generationID, sessionID)
	if err != nil {
		return GenerationView{}, fmt.Errorf("list image generation assets: %w", err)
	}

	return GenerationView{
		Generation: mapGeneration(generationRecord),
		Assets:     mapAssets(assetRecords),
	}, nil
}

func (s *Service) ExecuteGeneration(ctx context.Context, generationID string) error {
	if err := validateUUID(generationID, "generationId"); err != nil {
		return err
	}
	if s.provider == nil {
		return fmt.Errorf("image generation provider is not configured")
	}
	if s.blobs == nil {
		return fmt.Errorf("blob store is not configured")
	}
	if s.beginWriteTxFn == nil || s.generationRead == nil || s.assetRead == nil {
		return fmt.Errorf("image generation service is not configured")
	}

	generation, err := s.generationRead.GetByID(ctx, generationID)
	if err != nil {
		return mapRepositoryError(err, "image generation not found")
	}

	switch Status(generation.Status) {
	case StatusPending:
		return s.executePendingGeneration(ctx, generation)
	case StatusRunning:
		return s.executeRunningGeneration(ctx, generation)
	case StatusCompleted, StatusFailed:
		return nil
	default:
		return fmt.Errorf("unsupported image generation status %q", generation.Status)
	}
}

func (s *Service) executePendingGeneration(ctx context.Context, generation repository.ImageGeneration) error {
	providerName, providerModel := s.resolvedProviderMetadata(generation)
	claimed, err := s.generationRead.ClaimPending(ctx, repository.ClaimPendingImageGenerationParams{
		ID:            generation.ID,
		ProviderName:  providerName,
		ProviderModel: providerModel,
	})
	if err != nil {
		return fmt.Errorf("claim pending image generation: %w", err)
	}
	if !claimed {
		return nil
	}
	generation.ProviderName = providerName
	generation.ProviderModel = providerModel
	generation.Status = string(StatusRunning)

	inputAssets, err := s.assetRead.ListByGeneration(ctx, generation.ID)
	if err != nil {
		return s.failGenerationWithCause(ctx, generation, "list image generation assets", "input_asset_read_failed", err)
	}

	inputImages, err := s.loadInputImages(ctx, inputAssets)
	if err != nil {
		return s.failGenerationWithCause(ctx, generation, "load input images", "input_asset_read_failed", err)
	}

	result, err := s.provider.Generate(ctx, buildGenerateRequest(generation, inputImages))
	if err != nil {
		return s.failGenerationWithCause(ctx, generation, "generate image", providerErrorCode(err), err)
	}

	if result.Completed() {
		return s.completeGeneration(ctx, generation, result.Images)
	}
	if result.Job == nil {
		return s.failGenerationWithMessage(ctx, generation, "invalid_provider_output", "provider returned neither output images nor a job")
	}

	jobID := strings.TrimSpace(result.Job.JobID)
	if jobID == "" {
		return s.failGenerationWithMessage(ctx, generation, "invalid_provider_output", "provider returned an async job without a job id")
	}

	generation.ProviderJobID = jobID
	if err := s.generationRead.UpdateState(ctx, repository.UpdateImageGenerationStateParams{
		ID:            generation.ID,
		ProviderName:  providerName,
		ProviderModel: providerModel,
		ProviderJobID: jobID,
		Status:        string(StatusRunning),
	}); err != nil {
		return s.failGenerationWithCause(ctx, generation, "persist image generation job metadata", "persist_generation_state_failed", err)
	}

	return nil
}

func (s *Service) executeRunningGeneration(ctx context.Context, generation repository.ImageGeneration) error {
	jobID := strings.TrimSpace(generation.ProviderJobID)
	if jobID == "" {
		if generation.StartedAt != nil && time.Since(*generation.StartedAt) < providerJobIDPersistGracePeriod {
			return nil
		}
		return s.failGenerationWithMessage(ctx, generation, "missing_provider_job_id", "provider job id was not persisted after starting image generation")
	}

	pollResult, err := s.provider.Poll(ctx, imageprovider.ProviderJob{JobID: jobID})
	if err != nil {
		return s.failGenerationWithCause(ctx, generation, "poll image generation", providerErrorCode(err), err)
	}

	switch pollResult.Status {
	case imageprovider.PollStatusPending:
		return nil
	case imageprovider.PollStatusCompleted:
		return s.completeGeneration(ctx, generation, pollResult.Images)
	case imageprovider.PollStatusFailed:
		message := strings.TrimSpace(pollResult.Err)
		if message == "" {
			message = "provider job failed"
		}
		return s.failGenerationWithMessage(ctx, generation, "provider_job_failed", message)
	default:
		return s.failGenerationWithMessage(ctx, generation, "invalid_provider_output", fmt.Sprintf("unsupported provider poll status %q", pollResult.Status))
	}
}

func (s *Service) completeGeneration(ctx context.Context, generation repository.ImageGeneration, images []imageprovider.OutputImage) error {
	if len(images) != 1 {
		return s.failGenerationWithMessage(ctx, generation, "invalid_provider_output", fmt.Sprintf("provider returned %d output images; expected exactly 1", len(images)))
	}

	outputAsset, err := s.prepareOutputAsset(ctx, images[0])
	if err != nil {
		return s.failGenerationWithCause(ctx, generation, "prepare output image", "invalid_provider_output", err)
	}

	tx, err := s.beginWriteTxFn(ctx)
	if err != nil {
		return s.failGenerationWithCause(ctx, generation, "begin output persistence transaction", "persist_output_failed", err)
	}
	lockedGeneration, err := tx.LockGenerationForUpdate(ctx, generation.ID)
	if err != nil {
		_ = tx.Rollback(ctx)
		return s.failGenerationWithCause(ctx, generation, "lock image generation for output persistence", "persist_output_failed", err)
	}
	if Status(lockedGeneration.Status) != StatusRunning || lockedGeneration.CompletedAt != nil {
		_ = tx.Rollback(ctx)
		return nil
	}
	generation = lockedGeneration

	blobPath := buildOutputBlobPath(generation.SessionID, generation.ID, outputAsset.Filename)
	if err := s.blobs.Upload(ctx, blobPath, outputAsset.Data, outputAsset.MediaType); err != nil {
		_ = tx.Rollback(ctx)
		return s.failGenerationWithCause(ctx, generation, "store output image", "store_output_failed", err)
	}
	rollbackAndCleanup := func() {
		_ = tx.Rollback(ctx)
		s.cleanupBlobs([]string{blobPath})
	}

	if _, err := tx.CreateGenerationAsset(ctx, repository.CreateImageGenerationAssetParams{
		GenerationID: generation.ID,
		Role:         string(AssetRoleOutput),
		SortOrder:    0,
		BlobPath:     blobPath,
		MediaType:    outputAsset.MediaType,
		Filename:     outputAsset.Filename,
		SizeBytes:    outputAsset.SizeBytes,
		Sha256:       outputAsset.SHA256,
		Width:        outputAsset.Width,
		Height:       outputAsset.Height,
	}); err != nil {
		rollbackAndCleanup()
		return s.failGenerationWithCause(ctx, generation, "persist output asset", "persist_output_failed", err)
	}

	completedAt := time.Now().UTC()
	providerName, providerModel := s.resolvedProviderMetadata(generation)
	if err := tx.UpdateGenerationState(ctx, repository.UpdateImageGenerationStateParams{
		ID:            generation.ID,
		ProviderName:  providerName,
		ProviderModel: providerModel,
		ProviderJobID: generation.ProviderJobID,
		Status:        string(StatusCompleted),
		CompletedAt:   &completedAt,
	}); err != nil {
		rollbackAndCleanup()
		return s.failGenerationWithCause(ctx, generation, "mark image generation completed", "persist_output_failed", err)
	}

	if err := tx.Commit(ctx); err != nil {
		rollbackAndCleanup()
		return s.failGenerationWithCause(ctx, generation, "commit output persistence transaction", "persist_output_failed", err)
	}

	return nil
}

func (s *Service) loadInputImages(ctx context.Context, assets []repository.ImageGenerationAsset) ([][]byte, error) {
	inputAssets := make([]repository.ImageGenerationAsset, 0, len(assets))
	for _, asset := range assets {
		if AssetRole(asset.Role) == AssetRoleInput {
			inputAssets = append(inputAssets, asset)
		}
	}

	sort.Slice(inputAssets, func(i, j int) bool {
		if inputAssets[i].SortOrder == inputAssets[j].SortOrder {
			return inputAssets[i].ID < inputAssets[j].ID
		}
		return inputAssets[i].SortOrder < inputAssets[j].SortOrder
	})

	images := make([][]byte, 0, len(inputAssets))
	maxReferenceImageBytes := s.maxReferenceImageBytes
	if maxReferenceImageBytes <= 0 {
		maxReferenceImageBytes = DefaultMaxReferenceImageBytes
	}
	for _, asset := range inputAssets {
		data, err := s.blobs.DownloadWithinLimit(ctx, asset.BlobPath, maxReferenceImageBytes)
		if err != nil {
			return nil, fmt.Errorf("download input asset %q: %w", asset.Filename, err)
		}
		images = append(images, data)
	}

	return images, nil
}

func buildGenerateRequest(generation repository.ImageGeneration, inputImages [][]byte) imageprovider.GenerateRequest {
	return imageprovider.GenerateRequest{
		Mode:         imageprovider.Mode(generation.Mode),
		Prompt:       generation.Prompt,
		Width:        int(generation.Width),
		Height:       int(generation.Height),
		OutputFormat: defaultOutputFormat,
		NumImages:    int(generation.RequestedImageCount),
		InputImages:  inputImages,
	}
}

func (s *Service) prepareOutputAsset(ctx context.Context, image imageprovider.OutputImage) (preparedAsset, error) {
	data, mediaType, err := s.resolveOutputImageData(ctx, image)
	if err != nil {
		return preparedAsset{}, err
	}

	mediaType = normalizeMediaType(mediaType)
	detectedMediaType := normalizeMediaType(http.DetectContentType(data))
	if mediaType == "" || mediaType == "application/octet-stream" {
		mediaType = detectedMediaType
	}
	if _, ok := supportedOutputMediaTypes[mediaType]; !ok {
		return preparedAsset{}, fmt.Errorf("unsupported output media type %q", mediaType)
	}

	sum := sha256.Sum256(data)
	width, height := detectImageDimensions(data)

	return preparedAsset{
		Filename:  outputFilename(mediaType),
		MediaType: mediaType,
		SizeBytes: int64(len(data)),
		SHA256:    hex.EncodeToString(sum[:]),
		Width:     width,
		Height:    height,
		Data:      data,
	}, nil
}

func (s *Service) resolveOutputImageData(ctx context.Context, image imageprovider.OutputImage) ([]byte, string, error) {
	if len(image.Data) > 0 {
		if int64(len(image.Data)) > maxOutputImageBytes {
			return nil, "", fmt.Errorf("inline output image exceeds %d bytes", maxOutputImageBytes)
		}
		return slices.Clone(image.Data), image.MediaType, nil
	}

	outputURL, err := validateOutputImageURL(image.URL)
	if err != nil {
		return nil, "", err
	}
	if err := validateOutputImageHost(ctx, s.outputResolver, outputURL.Hostname()); err != nil {
		return nil, "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, outputURL.String(), nil)
	if err != nil {
		return nil, "", fmt.Errorf("create output download request: %w", err)
	}

	resp, err := s.outputHTTPClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("download output image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("download output image: unexpected status %d", resp.StatusCode)
	}

	data, err := readBytesWithinLimit(resp.Body, maxOutputImageBytes)
	if err != nil {
		return nil, "", fmt.Errorf("read output image: %w", err)
	}

	return data, resp.Header.Get("Content-Type"), nil
}

func (s *Service) failGenerationWithCause(ctx context.Context, generation repository.ImageGeneration, action string, code string, cause error) error {
	if failErr := s.persistGenerationFailure(ctx, generation, code, cause.Error()); failErr != nil {
		return fmt.Errorf("%s: %w (also failed to persist image generation failure: %v)", action, cause, failErr)
	}

	return fmt.Errorf("%s: %w", action, cause)
}

func (s *Service) failGenerationWithMessage(ctx context.Context, generation repository.ImageGeneration, code string, message string) error {
	if err := s.persistGenerationFailure(ctx, generation, code, message); err != nil {
		return err
	}

	return fmt.Errorf("%s", strings.TrimSpace(message))
}

func (s *Service) persistGenerationFailure(ctx context.Context, generation repository.ImageGeneration, code string, message string) error {
	completedAt := time.Now().UTC()
	providerName, providerModel := s.resolvedProviderMetadata(generation)
	if err := s.generationRead.UpdateState(ctx, repository.UpdateImageGenerationStateParams{
		ID:            generation.ID,
		ProviderName:  providerName,
		ProviderModel: providerModel,
		ProviderJobID: generation.ProviderJobID,
		Status:        string(StatusFailed),
		ErrorCode:     strings.TrimSpace(code),
		ErrorMessage:  strings.TrimSpace(message),
		CompletedAt:   &completedAt,
	}); err != nil {
		return fmt.Errorf("mark image generation failed: %w", err)
	}

	return nil
}

func (s *Service) providerName() string {
	if metadata, ok := s.provider.(imageprovider.ProviderMetadata); ok {
		return strings.TrimSpace(metadata.ProviderName())
	}
	return ""
}

func (s *Service) providerModel() string {
	if metadata, ok := s.provider.(imageprovider.ProviderMetadata); ok {
		return strings.TrimSpace(metadata.ProviderModel())
	}
	return ""
}

// resolvedProviderMetadata preserves the metadata already persisted on the
// generation when the current provider implementation does not expose
// ProviderMetadata, keeping later running/completed/failed transitions stable.
// Stored values are trimmed as a final normalization step before rewriting them.
func (s *Service) resolvedProviderMetadata(generation repository.ImageGeneration) (string, string) {
	providerName := s.providerName()
	if providerName == "" {
		providerName = strings.TrimSpace(generation.ProviderName)
	}

	providerModel := s.providerModel()
	if providerModel == "" {
		providerModel = strings.TrimSpace(generation.ProviderModel)
	}

	return providerName, providerModel
}

func providerErrorCode(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "provider_timeout"
	}

	var apiErr imageprovider.APIError
	if errors.As(err, &apiErr) {
		return fmt.Sprintf("provider_http_%d", apiErr.StatusCode)
	}

	// Check for pointer-type APIError as well, since the value-target check above
	// handles imageprovider.APIError values but does not match *APIError pointers.
	var apiErrPtr *imageprovider.APIError
	if errors.As(err, &apiErrPtr) && apiErrPtr != nil {
		return fmt.Sprintf("provider_http_%d", apiErrPtr.StatusCode)
	}

	return "provider_request_failed"
}

func readBytesWithinLimit(r io.Reader, maxBytes int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("content exceeds %d bytes", maxBytes)
	}

	return data, nil
}

func newOutputHTTPClient(timeout time.Duration, resolver hostnameResolver) *http.Client {
	dialer := &net.Dialer{Timeout: timeout}

	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy: func(*http.Request) (*url.URL, error) { return nil, nil },
			DialContext: func(ctx context.Context, network string, address string) (net.Conn, error) {
				dialAddress, err := validatedOutputDialAddress(ctx, resolver, address)
				if err != nil {
					return nil, err
				}
				return dialer.DialContext(ctx, network, dialAddress)
			},
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func validatedOutputDialAddress(ctx context.Context, resolver hostnameResolver, address string) (string, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return "", err
	}

	if ip := net.ParseIP(strings.TrimSpace(host)); ip != nil {
		if err := validateOutputIP(ip); err != nil {
			return "", err
		}
		return net.JoinHostPort(ip.String(), port), nil
	}

	if strings.EqualFold(strings.TrimSpace(host), "localhost") {
		return "", fmt.Errorf("provider output URL host is not allowed")
	}

	resolver = effectiveOutputResolver(resolver)
	ips, err := resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return "", fmt.Errorf("resolve output image host: %w", err)
	}
	if len(ips) == 0 {
		return "", fmt.Errorf("provider output URL host did not resolve")
	}

	for _, ip := range ips {
		if err := validateOutputIP(ip.IP); err != nil {
			return "", err
		}
	}

	return net.JoinHostPort(ips[0].IP.String(), port), nil
}

func validateOutputImageURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed == nil {
		return nil, fmt.Errorf("provider output URL is invalid")
	}
	if !parsed.IsAbs() || parsed.Host == "" {
		return nil, fmt.Errorf("provider output URL must be an absolute HTTPS URL")
	}
	if !strings.EqualFold(parsed.Scheme, "https") {
		return nil, fmt.Errorf("provider output URL must use HTTPS")
	}
	if parsed.User != nil {
		return nil, fmt.Errorf("provider output URL is invalid")
	}

	hostname := parsed.Hostname()
	if hostname == "" {
		return nil, fmt.Errorf("provider output URL must include a host")
	}
	if strings.EqualFold(hostname, "localhost") {
		return nil, fmt.Errorf("provider output URL host is not allowed")
	}
	if ip := net.ParseIP(hostname); ip != nil {
		if err := validateOutputIP(ip); err != nil {
			return nil, err
		}
	}

	return parsed, nil
}

func validateOutputImageHost(ctx context.Context, resolver hostnameResolver, host string) error {
	if ip := net.ParseIP(strings.TrimSpace(host)); ip != nil {
		return validateOutputIP(ip)
	}

	if strings.EqualFold(strings.TrimSpace(host), "localhost") {
		return fmt.Errorf("provider output URL host is not allowed")
	}

	resolver = effectiveOutputResolver(resolver)
	ips, err := resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Errorf("resolve output image host: %w", err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("provider output URL host did not resolve")
	}
	for _, ip := range ips {
		if err := validateOutputIP(ip.IP); err != nil {
			return err
		}
	}

	return nil
}

// effectiveOutputResolver keeps production behavior on net.DefaultResolver
// while allowing tests to inject deterministic hostname resolution.
func effectiveOutputResolver(resolver hostnameResolver) hostnameResolver {
	if resolver != nil {
		return resolver
	}
	return net.DefaultResolver
}

func validateOutputIP(ip net.IP) error {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return fmt.Errorf("provider output URL host is not allowed")
	}
	addr = addr.Unmap()

	if !addr.IsGlobalUnicast() || addr.IsPrivate() {
		return fmt.Errorf("provider output URL host is not allowed")
	}
	for _, prefix := range blockedOutputIPPrefixes {
		if prefix.Contains(addr) {
			return fmt.Errorf("provider output URL host is not allowed")
		}
	}

	return nil
}

type preparedAsset struct {
	Filename  string
	MediaType string
	SizeBytes int64
	SHA256    string
	Width     *int64
	Height    *int64
	Data      []byte
}

func validateCreateGenerationParams(params CreateGenerationParams, maxReferenceImageBytes int64) (Resolution, []preparedAsset, string, error) {
	prompt := strings.TrimSpace(params.Prompt)
	if prompt == "" {
		return Resolution{}, nil, "", ValidationError{
			StatusCode: http.StatusBadRequest,
			Message:    "prompt is required",
		}
	}

	resolution, ok := resolutionByKey(strings.TrimSpace(params.ResolutionKey))
	if !ok {
		return Resolution{}, nil, "", ValidationError{
			StatusCode: http.StatusBadRequest,
			Message:    fmt.Sprintf("resolution must be one of %s", supportedResolutionKeys()),
		}
	}

	switch params.Mode {
	case ModeTextToImage:
		if len(params.ReferenceImages) > 0 {
			return Resolution{}, nil, "", ValidationError{
				StatusCode: http.StatusBadRequest,
				Message:    "reference images are only supported for image_edit mode",
			}
		}
	case ModeImageEdit:
		if len(params.ReferenceImages) == 0 {
			return Resolution{}, nil, "", ValidationError{
				StatusCode: http.StatusBadRequest,
				Message:    "image_edit mode requires at least one reference image",
			}
		}
	default:
		return Resolution{}, nil, "", ValidationError{
			StatusCode: http.StatusBadRequest,
			Message:    "mode must be one of text_to_image or image_edit",
		}
	}

	if len(params.ReferenceImages) > MaxReferenceImages {
		return Resolution{}, nil, "", ValidationError{
			StatusCode: http.StatusBadRequest,
			Message:    fmt.Sprintf("too many reference images: maximum %d files", MaxReferenceImages),
		}
	}

	preparedAssets := make([]preparedAsset, 0, len(params.ReferenceImages))
	for _, upload := range params.ReferenceImages {
		asset, err := prepareInputAsset(upload, maxReferenceImageBytes)
		if err != nil {
			return Resolution{}, nil, "", err
		}

		preparedAssets = append(preparedAssets, asset)
	}

	return resolution, preparedAssets, prompt, nil
}

func prepareInputAsset(upload InputAssetUpload, maxReferenceImageBytes int64) (preparedAsset, error) {
	filename := filenames.Sanitize(upload.Filename, defaultAssetFilename)
	if len(upload.Data) == 0 {
		return preparedAsset{}, ValidationError{
			StatusCode: http.StatusBadRequest,
			Message:    fmt.Sprintf("reference image %q is empty", filename),
		}
	}

	if maxReferenceImageBytes > 0 && int64(len(upload.Data)) > maxReferenceImageBytes {
		return preparedAsset{}, ValidationError{
			StatusCode: http.StatusRequestEntityTooLarge,
			Message:    fmt.Sprintf("reference image %q exceeds %d bytes", filename, maxReferenceImageBytes),
		}
	}

	mediaType := http.DetectContentType(upload.Data)
	if _, ok := supportedInputMediaTypes[mediaType]; !ok {
		return preparedAsset{}, ValidationError{
			StatusCode: http.StatusUnsupportedMediaType,
			Message:    fmt.Sprintf("reference image %q has unsupported media type %q", filename, mediaType),
		}
	}

	sum := sha256.Sum256(upload.Data)
	width, height := detectImageDimensions(upload.Data)

	return preparedAsset{
		Filename:  filename,
		MediaType: mediaType,
		SizeBytes: int64(len(upload.Data)),
		SHA256:    hex.EncodeToString(sum[:]),
		Width:     width,
		Height:    height,
		Data:      upload.Data,
	}, nil
}

func buildInputBlobPath(sessionID, generationID string, index int, asset preparedAsset) string {
	return fmt.Sprintf(
		"sessions/%s/image-generations/%s/inputs/%02d-%s-%s",
		sessionID,
		generationID,
		index+1,
		asset.SHA256[:16],
		asset.Filename,
	)
}

func buildOutputBlobPath(sessionID, generationID, filename string) string {
	return fmt.Sprintf(
		"sessions/%s/image-generations/%s/outputs/%s",
		sessionID,
		generationID,
		filenames.Sanitize(filename, defaultAssetFilename),
	)
}

func outputFilename(mediaType string) string {
	switch mediaType {
	case "image/png":
		return defaultOutputFilenamePrefix + ".png"
	case "image/jpeg":
		return defaultOutputFilenamePrefix + ".jpg"
	case "image/webp":
		return defaultOutputFilenamePrefix + ".webp"
	}

	extensions, _ := mime.ExtensionsByType(mediaType)
	if len(extensions) == 0 {
		return defaultOutputFilenamePrefix
	}

	return defaultOutputFilenamePrefix + extensions[0]
}

func normalizeMediaType(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if idx := strings.Index(value, ";"); idx >= 0 {
		value = value[:idx]
	}

	return strings.ToLower(value)
}

func resolutionByKey(key string) (Resolution, bool) {
	for _, resolution := range supportedResolutionCatalog {
		if resolution.Key == key {
			return resolution, true
		}
	}

	return Resolution{}, false
}

func supportedResolutionKeys() string {
	keys := make([]string, 0, len(supportedResolutionCatalog))
	for _, resolution := range supportedResolutionCatalog {
		keys = append(keys, resolution.Key)
	}

	return strings.Join(keys, ", ")
}

func detectImageDimensions(data []byte) (*int64, *int64) {
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, nil
	}

	width := int64(config.Width)
	height := int64(config.Height)

	return &width, &height
}

func mapGeneration(record repository.ImageGeneration) Generation {
	return Generation{
		ID:        record.ID,
		SessionID: record.SessionID,
		Mode:      Mode(record.Mode),
		Prompt:    record.Prompt,
		Resolution: Resolution{
			Key:    record.ResolutionKey,
			Width:  record.Width,
			Height: record.Height,
		},
		OutputImageCount: record.RequestedImageCount,
		ProviderName:     record.ProviderName,
		ProviderModel:    record.ProviderModel,
		ProviderJobID:    record.ProviderJobID,
		Status:           Status(record.Status),
		ErrorCode:        record.ErrorCode,
		ErrorMessage:     record.ErrorMessage,
		CreatedAt:        record.CreatedAt,
		CompletedAt:      record.CompletedAt,
	}
}

func mapAssets(records []repository.ImageGenerationAsset) []Asset {
	assets := make([]Asset, 0, len(records))
	for _, record := range records {
		assets = append(assets, Asset{
			ID:           record.ID,
			GenerationID: record.GenerationID,
			Role:         AssetRole(record.Role),
			SortOrder:    record.SortOrder,
			BlobPath:     record.BlobPath,
			MediaType:    record.MediaType,
			Filename:     record.Filename,
			SizeBytes:    record.SizeBytes,
			SHA256:       record.Sha256,
			Width:        record.Width,
			Height:       record.Height,
			CreatedAt:    record.CreatedAt,
		})
	}

	return assets
}

func validateUUID(value, field string) error {
	var uuid pgtype.UUID
	if err := uuid.Scan(strings.TrimSpace(value)); err != nil {
		return ValidationError{
			StatusCode: http.StatusBadRequest,
			Message:    fmt.Sprintf("%s must be a valid UUID", field),
		}
	}

	return nil
}

func mapRepositoryError(err error, notFoundMessage string) error {
	if errors.Is(err, repository.ErrNotFound) {
		return ValidationError{
			StatusCode: http.StatusNotFound,
			Message:    notFoundMessage,
		}
	}

	return err
}

func (s *Service) cleanupBlobs(blobPaths []string) {
	if s.blobs == nil || len(blobPaths) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, blobPath := range blobPaths {
		_ = s.blobs.Delete(ctx, blobPath)
	}
}

type repositoryWriteTx struct {
	tx pgx.Tx
}

func (t repositoryWriteTx) GetSession(ctx context.Context, sessionID string) (repository.Session, error) {
	return repository.NewSessionRepository(t.tx).GetByID(ctx, sessionID)
}

func (t repositoryWriteTx) CreateGeneration(ctx context.Context, params repository.CreateImageGenerationParams) (repository.ImageGeneration, error) {
	return repository.NewImageGenerationRepository(t.tx).Create(ctx, params)
}

func (t repositoryWriteTx) CreateGenerationAsset(ctx context.Context, params repository.CreateImageGenerationAssetParams) (repository.ImageGenerationAsset, error) {
	return repository.NewImageGenerationAssetRepository(t.tx).Create(ctx, params)
}

func (t repositoryWriteTx) LockGenerationForUpdate(ctx context.Context, generationID string) (repository.ImageGeneration, error) {
	return repository.NewImageGenerationRepository(t.tx).LockForUpdate(ctx, generationID)
}

func (t repositoryWriteTx) UpdateGenerationState(ctx context.Context, params repository.UpdateImageGenerationStateParams) error {
	return repository.NewImageGenerationRepository(t.tx).UpdateState(ctx, params)
}

func (t repositoryWriteTx) Commit(ctx context.Context) error {
	return t.tx.Commit(ctx)
}

func (t repositoryWriteTx) Rollback(ctx context.Context) error {
	return t.tx.Rollback(ctx)
}
