package presentationgen

import "time"

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
	AssetRoleInput  AssetRole = "input"
	AssetRoleOutput AssetRole = "output"
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

type Generation struct {
	ID            string
	SessionID     string
	Prompt        string
	ProviderName  string
	ProviderModel string
	ProviderJobID string
	Status        Status
	ErrorCode     string
	ErrorMessage  string
	CreatedAt     time.Time
	StartedAt     *time.Time
	CompletedAt   *time.Time
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
	CreatedAt    time.Time
}

type GenerationView struct {
	Generation Generation
	Assets     []Asset
}
