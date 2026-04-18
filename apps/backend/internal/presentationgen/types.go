package presentationgen

import (
	"time"

	"github.com/ivanlin/ulduar/apps/backend/internal/presentationdialect"
)

const (
	InputMediaTypeJPEG  = "image/jpeg"
	InputMediaTypePNG   = "image/png"
	InputMediaTypeWEBP  = "image/webp"
	InputMediaTypePDF   = "application/pdf"
	OutputMediaTypePPTX = "application/vnd.openxmlformats-officedocument.presentationml.presentation"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
)

type AssetRole string

const (
	AssetRoleInput    AssetRole = "input"
	AssetRoleResolved AssetRole = "resolved"
	AssetRoleOutput   AssetRole = "output"
)

type AssetSourceType string

const (
	AssetSourceTypeInputAsset  AssetSourceType = "input_asset"
	AssetSourceTypeThemeBundle AssetSourceType = "theme_bundle"
)

var supportedInputMediaTypes = []string{
	InputMediaTypeJPEG,
	InputMediaTypePNG,
	InputMediaTypeWEBP,
	InputMediaTypePDF,
}

func SupportedInputMediaTypes() []string {
	mediaTypes := make([]string, len(supportedInputMediaTypes))
	copy(mediaTypes, supportedInputMediaTypes)

	return mediaTypes
}

type PlannerConfig struct {
	Endpoint       string
	APIKey         string
	APIVersion     string
	Deployment     string
	SystemPrompt   string
	RequestTimeout time.Duration
	StreamTimeout  time.Duration
}

type InputAssetUpload struct {
	Filename string
	Data     []byte
}

type CreateGenerationParams struct {
	SessionID   string
	Prompt      string
	Attachments []InputAssetUpload
}

type Capabilities struct {
	InputMediaTypes []string
	OutputMediaType string
	ProviderName    string
	ThemePresets    []presentationdialect.ThemePresetMetadata
}

type Generation struct {
	ID                string
	SessionID         string
	Prompt            string
	PlannerOutputJSON []byte
	DialectJSON       []byte
	ProviderName      string
	ProviderModel     string
	ProviderJobID     string
	Status            Status
	ErrorCode         string
	ErrorMessage      string
	CreatedAt         time.Time
	StartedAt         *time.Time
	CompletedAt       *time.Time
}

type Asset struct {
	ID            string
	GenerationID  string
	Role          AssetRole
	AssetRef      string
	SourceType    AssetSourceType
	SourceAssetID string
	SourceRef     string
	SortOrder     int64
	BlobPath      string
	MediaType     string
	Filename      string
	SizeBytes     int64
	SHA256        string
	CreatedAt     time.Time
}

type GenerationView struct {
	Generation Generation
	Assets     []Asset
}

type AssetContent struct {
	Filename  string
	MediaType string
	Data      []byte
}
