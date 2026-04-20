package presentationgen

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/ivanlin/ulduar/apps/backend/internal/filenames"
	"github.com/ivanlin/ulduar/apps/backend/internal/presentationdialect"
	"github.com/ivanlin/ulduar/apps/backend/internal/repository"
)

var (
	errResolvedAssetUnsupportedRef = errors.New("presentation resolved asset reference unsupported")
	errResolvedAssetNotFound       = errors.New("presentation resolved asset reference not found")
	errThemeAssetUnavailable       = errors.New("presentation theme asset unavailable")
)

type AssetResolver interface {
	Resolve(ctx context.Context, params ResolveAssetsParams) (ResolveAssetsResult, error)
}

type ResolveAssetsParams struct {
	SessionID    string
	GenerationID string
	Document     presentationdialect.Document
	InputAssets  []repository.PresentationGenerationAsset
}

type ResolveAssetsResult struct {
	Assets           []repository.CreatePresentationGenerationAssetParams
	CleanupBlobPaths []string
}

type AttachmentAlias struct {
	AssetID   string
	AssetRef  string
	Filename  string
	MediaType string
}

type defaultAssetResolver struct {
	blobs BlobStore
}

type themeBundleAsset struct {
	filename  string
	mediaType string
	data      []byte
}

func newDefaultAssetResolver(blobs BlobStore) AssetResolver {
	return defaultAssetResolver{blobs: blobs}
}

func (r defaultAssetResolver) Resolve(ctx context.Context, params ResolveAssetsParams) (ResolveAssetsResult, error) {
	refs := collectDocumentAssetRefs(params.Document)
	if len(refs) == 0 {
		return ResolveAssetsResult{}, nil
	}

	aliases := attachmentAliases(params.InputAssets)
	inputAssetByID := make(map[string]repository.PresentationGenerationAsset, len(params.InputAssets))
	for _, asset := range params.InputAssets {
		inputAssetByID[asset.ID] = asset
	}
	aliasByRef := make(map[string]repository.PresentationGenerationAsset, len(aliases))
	for _, alias := range aliases {
		if asset, ok := inputAssetByID[alias.AssetID]; ok {
			aliasByRef[alias.AssetRef] = asset
		}
	}

	resolvedPresetID := presentationdialect.ResolveThemePresetID(dereferenceString(params.Document.ThemePresetID))
	result := ResolveAssetsResult{
		Assets:           make([]repository.CreatePresentationGenerationAssetParams, 0, len(refs)),
		CleanupBlobPaths: make([]string, 0, len(refs)),
	}
	for index, assetRef := range refs {
		sourceType, key, err := parseResolvedAssetRef(assetRef)
		if err != nil {
			return result, err
		}
		switch sourceType {
		case string(AssetSourceTypeInputAsset):
			asset, ok := aliasByRef[assetRef]
			if !ok {
				return result, fmt.Errorf("%w: %q is not one of the available attachment asset refs", errResolvedAssetNotFound, assetRef)
			}
			result.Assets = append(result.Assets, repository.CreatePresentationGenerationAssetParams{
				GenerationID:  params.GenerationID,
				Role:          string(AssetRoleResolved),
				AssetRef:      assetRef,
				SourceType:    string(AssetSourceTypeInputAsset),
				SourceAssetID: asset.ID,
				SortOrder:     int64(index),
				BlobPath:      asset.BlobPath,
				MediaType:     asset.MediaType,
				Filename:      asset.Filename,
				SizeBytes:     asset.SizeBytes,
				Sha256:        asset.Sha256,
			})
		case string(AssetSourceTypeThemeBundle):
			builtInAsset, ok := presentationdialect.LookupThemeBundleAsset(resolvedPresetID, key)
			if !ok {
				return result, fmt.Errorf("%w: %q for preset %q", errThemeAssetUnavailable, assetRef, resolvedPresetID)
			}
			themeAsset := themeBundleAsset{
				filename:  builtInAsset.Filename,
				mediaType: builtInAsset.MediaType,
				data:      builtInAsset.Data,
			}
			if r.blobs == nil {
				return result, fmt.Errorf("blob store is not configured")
			}
			blobPath, prepared := buildResolvedThemeAsset(params.SessionID, params.GenerationID, themeAsset)
			if err := r.blobs.Upload(ctx, blobPath, prepared.Data, prepared.MediaType); err != nil {
				return result, fmt.Errorf("store resolved theme asset %q: %w", assetRef, err)
			}
			result.CleanupBlobPaths = append(result.CleanupBlobPaths, blobPath)
			result.Assets = append(result.Assets, repository.CreatePresentationGenerationAssetParams{
				GenerationID: params.GenerationID,
				Role:         string(AssetRoleResolved),
				AssetRef:     assetRef,
				SourceType:   string(AssetSourceTypeThemeBundle),
				SourceRef:    resolvedPresetID + ":" + key,
				SortOrder:    int64(index),
				BlobPath:     blobPath,
				MediaType:    prepared.MediaType,
				Filename:     prepared.Filename,
				SizeBytes:    prepared.SizeBytes,
				Sha256:       prepared.SHA256,
			})
		default:
			return result, fmt.Errorf("%w: %q", errResolvedAssetUnsupportedRef, assetRef)
		}
	}

	return result, nil
}

func attachmentAliases(assets []repository.PresentationGenerationAsset) []AttachmentAlias {
	inputAssets := sortInputAssets(assets)
	aliases := make([]AttachmentAlias, 0, len(inputAssets))
	used := map[string]int{}
	for index, asset := range inputAssets {
		aliasBase := assetAliasBase(asset.Filename)
		if aliasBase == "" {
			aliasBase = fmt.Sprintf("attachment-%d", index+1)
		}
		alias := aliasBase
		used[aliasBase]++
		if used[aliasBase] > 1 {
			alias = fmt.Sprintf("%s-%d", aliasBase, used[aliasBase])
		}
		aliases = append(aliases, AttachmentAlias{
			AssetID:   asset.ID,
			AssetRef:  "attachment:" + alias,
			Filename:  asset.Filename,
			MediaType: asset.MediaType,
		})
	}
	return aliases
}

func attachmentAliasGuidance(assets []repository.PresentationGenerationAsset) string {
	aliases := attachmentAliases(assets)
	if len(aliases) == 0 {
		return ""
	}
	var builder strings.Builder
	builder.WriteString("Available uploaded attachment asset refs (use these exact values when setting assetRef):")
	for _, alias := range aliases {
		builder.WriteString("\n- ")
		builder.WriteString(alias.AssetRef)
		builder.WriteString(" => ")
		builder.WriteString(alias.Filename)
	}
	return builder.String()
}

func collectDocumentAssetRefs(document presentationdialect.Document) []string {
	seen := map[string]struct{}{}
	refs := make([]string, 0)
	for _, slide := range document.Slides {
		for _, block := range slide.Blocks {
			appendDocumentAssetRef(&refs, seen, dereferenceString(block.AssetRef))
		}
		for _, column := range slide.Columns {
			for _, block := range column.Blocks {
				appendDocumentAssetRef(&refs, seen, dereferenceString(block.AssetRef))
			}
		}
	}
	return refs
}

func appendDocumentAssetRef(refs *[]string, seen map[string]struct{}, ref string) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return
	}
	if _, ok := seen[ref]; ok {
		return
	}
	seen[ref] = struct{}{}
	*refs = append(*refs, ref)
}

func parseResolvedAssetRef(ref string) (string, string, error) {
	parts := strings.SplitN(strings.TrimSpace(ref), ":", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
		return "", "", fmt.Errorf("%w: %q", errResolvedAssetUnsupportedRef, ref)
	}
	switch strings.TrimSpace(parts[0]) {
	case "attachment":
		return string(AssetSourceTypeInputAsset), strings.TrimSpace(parts[1]), nil
	case "theme":
		return string(AssetSourceTypeThemeBundle), strings.TrimSpace(parts[1]), nil
	default:
		return "", "", fmt.Errorf("%w: %q", errResolvedAssetUnsupportedRef, ref)
	}
}

func assetAliasBase(filename string) string {
	base := strings.TrimSpace(strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename)))
	if base == "" {
		return ""
	}
	var builder strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(base) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			builder.WriteRune(r)
			lastDash = false
		case !lastDash:
			builder.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(builder.String(), "-")
}

func buildResolvedThemeAsset(sessionID, generationID string, asset themeBundleAsset) (string, preparedAsset) {
	sum := sha256.Sum256(asset.data)
	prepared := preparedAsset{
		Filename:  filenames.Sanitize(asset.filename, "theme-asset.png"),
		MediaType: asset.mediaType,
		SizeBytes: int64(len(asset.data)),
		SHA256:    hex.EncodeToString(sum[:]),
		Data:      append([]byte(nil), asset.data...),
	}
	blobPath := fmt.Sprintf(
		"sessions/%s/presentation-generations/%s/resolved/%s",
		sessionID,
		generationID,
		prepared.Filename,
	)
	return blobPath, prepared
}

func dereferenceString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
