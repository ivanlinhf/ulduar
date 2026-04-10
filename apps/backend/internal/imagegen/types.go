package imagegen

import (
	"time"

	"github.com/ivanlin/ulduar/apps/backend/internal/imageprovider"
)

type Mode = imageprovider.Mode

const (
	ModeTextToImage = imageprovider.ModeTextToImage
	ModeImageEdit   = imageprovider.ModeImageEdit
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
	AssetRoleInput  AssetRole = "input"
	AssetRoleOutput AssetRole = "output"
)

const (
	MaxReferenceImages                  = 4
	OutputImageCountV1                  = 1
	DefaultMaxReferenceImageBytes int64 = 20 << 20
	defaultAssetFilename                = "image"
)

type Resolution struct {
	Key    string
	Width  int64
	Height int64
}

var supportedResolutionCatalog = []Resolution{
	{Key: "1024x1024", Width: 1024, Height: 1024},
	{Key: "1152x896", Width: 1152, Height: 896},
	{Key: "896x1152", Width: 896, Height: 1152},
	{Key: "1344x768", Width: 1344, Height: 768},
	{Key: "768x1344", Width: 768, Height: 1344},
	{Key: "1536x1024", Width: 1536, Height: 1024},
	{Key: "1024x1536", Width: 1024, Height: 1536},
}

func SupportedResolutions() []Resolution {
	resolutions := make([]Resolution, len(supportedResolutionCatalog))
	copy(resolutions, supportedResolutionCatalog)

	return resolutions
}

type InputAssetUpload struct {
	Filename string
	Data     []byte
}

type CreateGenerationParams struct {
	SessionID       string
	Mode            Mode
	Prompt          string
	ResolutionKey   string
	ReferenceImages []InputAssetUpload
}

type Capabilities struct {
	Modes              []Mode
	Resolutions        []Resolution
	MaxReferenceImages int
	OutputImageCount   int
	ProviderName       string
}

type Generation struct {
	ID               string
	SessionID        string
	Mode             Mode
	Prompt           string
	Resolution       Resolution
	OutputImageCount int64
	ProviderName     string
	ProviderModel    string
	ProviderJobID    string
	Status           Status
	ErrorCode        string
	ErrorMessage     string
	CreatedAt        time.Time
	CompletedAt      *time.Time
}

type Asset struct {
	ID           string
	GenerationID string
	Role         AssetRole
	SortOrder    int64
	BlobPath     string
	MediaType    string
	Filename     string
	SizeBytes    int64
	SHA256       string
	Width        *int64
	Height       *int64
	CreatedAt    time.Time
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
