// Package imageprovider defines a provider-neutral interface and types for image generation.
// Concrete adapters (e.g. Azure Foundry FLUX) implement ImageProvider and keep all
// provider-specific details out of the domain layer.
package imageprovider

import "context"

// Mode identifies the type of image generation operation.
type Mode string

const (
	ModeTextToImage Mode = "text_to_image"
	ModeImageEdit   Mode = "image_edit"
)

// GenerateRequest holds provider-neutral parameters for an image generation call.
type GenerateRequest struct {
	// Mode selects the generation operation.
	Mode Mode
	// Prompt is the user-facing instruction.
	Prompt string
	// Width and Height are the desired output dimensions in pixels.
	Width  int
	Height int
	// OutputFormat is the desired MIME sub-type, e.g. "jpeg" or "png".
	OutputFormat string
	// NumImages is the number of images to generate (defaults to 1 when zero).
	NumImages int
	// InputImages holds raw image bytes used as references in image_edit mode.
	// The adapter is responsible for encoding them into the provider's expected form.
	InputImages [][]byte
}

// OutputImage holds a single generated image returned by a provider.
// Exactly one of Data or URL will be populated.
type OutputImage struct {
	// Data contains the decoded image bytes when the provider delivers inline content.
	Data []byte
	// URL is a reference URL when the provider delivers a link instead of inline data.
	URL string
	// MediaType is the MIME type of the image, e.g. "image/jpeg".
	MediaType string
}

// ProviderJob represents an in-flight asynchronous generation job.
type ProviderJob struct {
	// JobID is the provider-assigned identifier for the in-flight job.
	JobID string
	// PollingURL is the endpoint to query for status updates.
	// When empty, the adapter constructs the polling URL from JobID.
	PollingURL string
}

// GenerateResult is returned by Generate.
// Exactly one of Images or Job will be populated.
type GenerateResult struct {
	// Images holds the completed output when generation finished synchronously.
	Images []OutputImage
	// Job is set when the provider accepted the request as an asynchronous job.
	Job *ProviderJob
}

// Completed reports whether the result is already resolved and no polling is required.
func (r GenerateResult) Completed() bool { return r.Job == nil }

// PollStatus describes the current state of an in-flight async job.
type PollStatus string

const (
	PollStatusPending   PollStatus = "pending"
	PollStatusCompleted PollStatus = "completed"
	PollStatusFailed    PollStatus = "failed"
)

// PollResult is returned by Poll.
type PollResult struct {
	Status PollStatus
	Images []OutputImage
	// Err carries a provider-reported error message when Status is PollStatusFailed.
	Err string
}

// ImageProvider is the provider-neutral interface for image generation.
type ImageProvider interface {
	// Generate submits an image generation request.
	// It returns either an immediately completed result or a ProviderJob for async polling.
	Generate(ctx context.Context, req GenerateRequest) (GenerateResult, error)
	// Poll queries the status of a previously submitted async job.
	Poll(ctx context.Context, job ProviderJob) (PollResult, error)
}
