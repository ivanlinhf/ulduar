// Package azurefoundry implements an imageprovider.ImageProvider backed by the
// Azure AI Foundry FLUX.2-pro model from Black Forest Labs.
//
// URLs are built as:
//
//	{endpoint}/providers/blackforestlabs/v1/{model-path}?api-version={version}
//
// Requests are authenticated with "Authorization: Bearer <api-key>".
package azurefoundry

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ivanlin/ulduar/apps/backend/internal/imageprovider"
)

const (
	DefaultAPIVersion    = "preview"
	DefaultModel         = "FLUX.2-pro"
	DefaultModelPath     = "flux-2-pro"
	DefaultTimeout       = 60 * time.Second
	ProviderName         = "azure_foundry"
	providerPathPrefix   = "/providers/blackforestlabs/v1"
	maxResponseBodyBytes = 16 * 1024 * 1024 // 16 MiB
)

// HTTPDoer is a small interface for the HTTP transport, enabling test doubles.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Client is an Azure Foundry FLUX adapter that implements imageprovider.ImageProvider.
type Client struct {
	endpoint   string // base URL, no trailing slash
	apiKey     string
	apiVersion string
	model      string
	modelPath  string
	httpClient HTTPDoer
}

// ClientOptions customizes a Client beyond the required endpoint and API key.
type ClientOptions struct {
	// APIVersion overrides the api-version query parameter (default: "preview").
	APIVersion string
	// Model overrides the model name sent in request payloads (default: "FLUX.2-pro").
	Model string
	// ModelPath overrides the URL path segment (default: "flux-2-pro").
	ModelPath string
	// RequestTimeout sets the HTTP client timeout (default: 60s).
	// Ignored when HTTPClient is also set (HTTPClient takes precedence).
	RequestTimeout time.Duration
	// HTTPClient replaces the default http.Client, useful in tests.
	// When set, RequestTimeout is ignored.
	HTTPClient HTTPDoer
}

// NewClient creates a FLUX adapter. endpoint and apiKey are required.
// Optional ClientOptions fields take effect when a non-zero options value is provided.
func NewClient(endpoint, apiKey string, opts ...ClientOptions) (*Client, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return nil, fmt.Errorf("azure foundry endpoint must not be empty")
	}
	normalized, err := normalizeEndpoint(endpoint)
	if err != nil {
		return nil, err
	}
	trimmedKey := strings.TrimSpace(apiKey)
	if trimmedKey == "" {
		return nil, fmt.Errorf("azure foundry api key must not be empty")
	}

	c := &Client{
		endpoint:   normalized,
		apiKey:     trimmedKey,
		apiVersion: DefaultAPIVersion,
		model:      DefaultModel,
		modelPath:  DefaultModelPath,
		httpClient: &http.Client{Timeout: DefaultTimeout},
	}

	if len(opts) > 0 {
		o := opts[0]
		if apiVersion := strings.TrimSpace(o.APIVersion); apiVersion != "" {
			c.apiVersion = apiVersion
		}
		if model := strings.TrimSpace(o.Model); model != "" {
			c.model = model
		}
		if modelPath := strings.TrimSpace(o.ModelPath); modelPath != "" {
			c.modelPath = modelPath
		}
		if o.RequestTimeout > 0 {
			c.httpClient = &http.Client{Timeout: o.RequestTimeout}
		}
		if o.HTTPClient != nil {
			c.httpClient = o.HTTPClient
		}
	}

	return c, nil
}

// Endpoint returns the base endpoint URL used by this client.
func (c *Client) Endpoint() string { return c.endpoint }

// Model returns the model name used in generation requests.
func (c *Client) Model() string { return c.model }

// ProviderName returns the stable provider identifier for persistence/debugging.
func (c *Client) ProviderName() string { return ProviderName }

// ProviderModel returns the configured provider model name for persistence/debugging.
func (c *Client) ProviderModel() string { return c.model }

// generateURL returns the full generation URL.
func (c *Client) generateURL() string {
	return c.operationURL(c.modelPath)
}

// jobURL returns the polling URL for an async job.
// It prefers a validated job.PollingURL when present, otherwise constructs one
// from JobID.
func (c *Client) jobURL(job imageprovider.ProviderJob) (string, error) {
	if pollingURL := c.validatedPollingURL(job.PollingURL); pollingURL != "" {
		return pollingURL, nil
	}

	jobID := strings.TrimSpace(job.JobID)
	if jobID == "" {
		return "", fmt.Errorf("provider job must include a job id when polling URL is absent or invalid")
	}

	return c.operationURL(c.modelPath, jobID), nil
}

// ---- provider-side JSON types ----

type textToImageRequest struct {
	Prompt       string `json:"prompt"`
	Width        int    `json:"width,omitempty"`
	Height       int    `json:"height,omitempty"`
	OutputFormat string `json:"output_format,omitempty"`
	NumImages    int    `json:"num_images,omitempty"`
	Model        string `json:"model,omitempty"`
}

type imageEditRequest struct {
	Prompt       string   `json:"prompt"`
	ImagePrompt  string   `json:"image_prompt,omitempty"`  // single base64-encoded image
	ImagePrompts []string `json:"image_prompts,omitempty"` // multiple base64-encoded images
	Width        int      `json:"width,omitempty"`
	Height       int      `json:"height,omitempty"`
	OutputFormat string   `json:"output_format,omitempty"`
	NumImages    int      `json:"num_images,omitempty"`
	Model        string   `json:"model,omitempty"`
}

// fluxImage represents one image entry in a synchronous provider response.
type fluxImage struct {
	URL         string `json:"url"`          // data: URL with inline base64, or a plain HTTPS URL
	ContentType string `json:"content_type"` // optional MIME type hint
}

// syncResponse is the response body for a 200/201 (immediate completion).
type syncResponse struct {
	Images []fluxImage `json:"images"`
	Prompt string      `json:"prompt,omitempty"`
}

type openAIImage struct {
	B64JSON string `json:"b64_json"`
	URL     string `json:"url"`
}

type openAIResponse struct {
	Data []openAIImage `json:"data"`
}

// asyncResponse is the response body for a 202 Accepted (async job queued).
type asyncResponse struct {
	ID          string   `json:"id"`
	Status      string   `json:"status"`
	FutureLinks []string `json:"future_links"`
}

// pollResponse is the response body returned when polling a job.
type pollResponse struct {
	ID     string      `json:"id"`
	Status string      `json:"status"`
	Result *pollResult `json:"result,omitempty"`
	Error  string      `json:"error,omitempty"`
}

type pollResult struct {
	Sample string `json:"sample"`
}

// APIError is kept as an alias for backwards compatibility in package tests.
type APIError = imageprovider.APIError

// ---- Generate ----

// Generate implements imageprovider.ImageProvider.
func (c *Client) Generate(ctx context.Context, req imageprovider.GenerateRequest) (imageprovider.GenerateResult, error) {
	var payload any
	switch req.Mode {
	case imageprovider.ModeTextToImage:
		payload = c.buildTextToImagePayload(req)
	case imageprovider.ModeImageEdit:
		payload = c.buildImageEditPayload(req)
	default:
		return imageprovider.GenerateResult{}, fmt.Errorf("unsupported image generation mode: %q", req.Mode)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return imageprovider.GenerateResult{}, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.generateURL(), bytes.NewReader(body))
	if err != nil {
		return imageprovider.GenerateResult{}, fmt.Errorf("create request: %w", err)
	}
	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return imageprovider.GenerateResult{}, fmt.Errorf("perform request: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodyBytes+1))
	if err != nil {
		return imageprovider.GenerateResult{}, fmt.Errorf("read response body: %w", err)
	}
	if len(rawBody) > maxResponseBodyBytes {
		return imageprovider.GenerateResult{}, fmt.Errorf("response body too large: exceeds %d bytes", maxResponseBodyBytes)
	}

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated:
		return c.normalizeSyncResponse(rawBody)
	case http.StatusAccepted:
		return c.normalizeAsyncResponse(rawBody)
	default:
		return imageprovider.GenerateResult{}, imageprovider.APIError{
			StatusCode: resp.StatusCode,
			Message:    strings.TrimSpace(string(rawBody)),
		}
	}
}

// ---- Poll ----

// Poll implements imageprovider.ImageProvider.
func (c *Client) Poll(ctx context.Context, job imageprovider.ProviderJob) (imageprovider.PollResult, error) {
	pollURL, err := c.jobURL(job)
	if err != nil {
		return imageprovider.PollResult{}, fmt.Errorf("resolve poll URL: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, pollURL, nil)
	if err != nil {
		return imageprovider.PollResult{}, fmt.Errorf("create poll request: %w", err)
	}
	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return imageprovider.PollResult{}, fmt.Errorf("perform poll request: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodyBytes+1))
	if err != nil {
		return imageprovider.PollResult{}, fmt.Errorf("read poll response body: %w", err)
	}
	if len(rawBody) > maxResponseBodyBytes {
		return imageprovider.PollResult{}, fmt.Errorf("poll response body too large: exceeds %d bytes", maxResponseBodyBytes)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return imageprovider.PollResult{}, imageprovider.APIError{
			StatusCode: resp.StatusCode,
			Message:    strings.TrimSpace(string(rawBody)),
		}
	}

	var pr pollResponse
	if err := json.Unmarshal(rawBody, &pr); err != nil {
		return imageprovider.PollResult{}, fmt.Errorf("decode poll response: %w", err)
	}

	switch strings.ToLower(pr.Status) {
	case "ready":
		if pr.Result == nil {
			return imageprovider.PollResult{Status: imageprovider.PollStatusCompleted}, nil
		}
		img, err := decodeImageField(pr.Result.Sample)
		if err != nil {
			return imageprovider.PollResult{}, fmt.Errorf("normalize poll image: %w", err)
		}
		return imageprovider.PollResult{
			Status: imageprovider.PollStatusCompleted,
			Images: []imageprovider.OutputImage{img},
		}, nil
	case "error", "failed":
		return imageprovider.PollResult{Status: imageprovider.PollStatusFailed, Err: pr.Error}, nil
	default:
		return imageprovider.PollResult{Status: imageprovider.PollStatusPending}, nil
	}
}

// ---- helpers ----

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
}

func (c *Client) operationURL(pathSegments ...string) string {
	base := c.endpoint + providerPathPrefix
	for _, segment := range pathSegments {
		base += "/" + url.PathEscape(strings.TrimSpace(segment))
	}

	parsed, _ := url.Parse(base)
	query := parsed.Query()
	query.Set("api-version", c.apiVersion)
	parsed.RawQuery = query.Encode()

	return parsed.String()
}

func (c *Client) validatedPollingURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	pollURL, err := url.Parse(raw)
	if err != nil || !pollURL.IsAbs() {
		return ""
	}
	if pollURL.Scheme != "http" && pollURL.Scheme != "https" {
		return ""
	}
	if pollURL.Host == "" || pollURL.User != nil {
		return ""
	}

	endpointURL, err := url.Parse(c.endpoint)
	if err != nil || !endpointURL.IsAbs() {
		return ""
	}
	if !strings.EqualFold(pollURL.Scheme, endpointURL.Scheme) {
		return ""
	}
	if !strings.EqualFold(pollURL.Host, endpointURL.Host) {
		return ""
	}

	return pollURL.String()
}

func (c *Client) buildTextToImagePayload(req imageprovider.GenerateRequest) textToImageRequest {
	n := req.NumImages
	if n == 0 {
		n = 1
	}
	return textToImageRequest{
		Prompt:       req.Prompt,
		Width:        req.Width,
		Height:       req.Height,
		OutputFormat: req.OutputFormat,
		NumImages:    n,
		Model:        c.model,
	}
}

func (c *Client) buildImageEditPayload(req imageprovider.GenerateRequest) imageEditRequest {
	n := req.NumImages
	if n == 0 {
		n = 1
	}
	r := imageEditRequest{
		Prompt:       req.Prompt,
		Width:        req.Width,
		Height:       req.Height,
		OutputFormat: req.OutputFormat,
		NumImages:    n,
		Model:        c.model,
	}
	switch len(req.InputImages) {
	case 0:
		// no reference images
	case 1:
		r.ImagePrompt = base64.StdEncoding.EncodeToString(req.InputImages[0])
	default:
		r.ImagePrompts = make([]string, len(req.InputImages))
		for i, img := range req.InputImages {
			r.ImagePrompts[i] = base64.StdEncoding.EncodeToString(img)
		}
	}
	return r
}

func (c *Client) normalizeSyncResponse(rawBody []byte) (imageprovider.GenerateResult, error) {
	var sr syncResponse
	if err := json.Unmarshal(rawBody, &sr); err != nil {
		return imageprovider.GenerateResult{}, fmt.Errorf("decode sync response: %w", err)
	}

	if len(sr.Images) > 0 {
		images := make([]imageprovider.OutputImage, 0, len(sr.Images))
		for _, img := range sr.Images {
			out, err := decodeImageField(img.URL)
			if err != nil {
				return imageprovider.GenerateResult{}, fmt.Errorf("normalize image: %w", err)
			}
			if out.MediaType == "" && img.ContentType != "" {
				out.MediaType = img.ContentType
			}
			images = append(images, out)
		}
		return imageprovider.GenerateResult{Images: images}, nil
	}

	var or openAIResponse
	if err := json.Unmarshal(rawBody, &or); err != nil {
		return imageprovider.GenerateResult{}, fmt.Errorf("decode alternate sync response: %w", err)
	}

	images := make([]imageprovider.OutputImage, 0, len(or.Data))
	for _, img := range or.Data {
		switch {
		case strings.TrimSpace(img.B64JSON) != "":
			decoded, err := base64.StdEncoding.DecodeString(img.B64JSON)
			if err != nil {
				return imageprovider.GenerateResult{}, fmt.Errorf("decode b64_json image data: %w", err)
			}
			images = append(images, imageprovider.OutputImage{Data: decoded})
		case strings.TrimSpace(img.URL) != "":
			out, err := decodeImageField(img.URL)
			if err != nil {
				return imageprovider.GenerateResult{}, fmt.Errorf("normalize data image: %w", err)
			}
			images = append(images, out)
		}
	}

	return imageprovider.GenerateResult{Images: images}, nil
}

func (c *Client) normalizeAsyncResponse(rawBody []byte) (imageprovider.GenerateResult, error) {
	var ar asyncResponse
	if err := json.Unmarshal(rawBody, &ar); err != nil {
		return imageprovider.GenerateResult{}, fmt.Errorf("decode async response: %w", err)
	}

	jobID := strings.TrimSpace(ar.ID)
	if jobID == "" {
		return imageprovider.GenerateResult{}, fmt.Errorf("decode async response: missing job id")
	}

	job := &imageprovider.ProviderJob{JobID: jobID}
	if len(ar.FutureLinks) > 0 {
		pollingURL := strings.TrimSpace(ar.FutureLinks[0])
		parsed, err := url.Parse(pollingURL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return imageprovider.GenerateResult{}, fmt.Errorf("decode async response: invalid future_links[0]: must be an absolute http/https URL")
		}
		job.PollingURL = pollingURL
	}

	return imageprovider.GenerateResult{Job: job}, nil
}

// decodeImageField normalizes a single image field that may be a data: URL (inline base64)
// or a plain http/https URL.
func decodeImageField(value string) (imageprovider.OutputImage, error) {
	if strings.HasPrefix(value, "data:") {
		// data:<mediaType>;base64,<encoded>
		withoutScheme := strings.TrimPrefix(value, "data:")
		meta, encoded, ok := strings.Cut(withoutScheme, ",")
		if !ok {
			return imageprovider.OutputImage{}, fmt.Errorf("malformed data URL: missing comma separator")
		}
		if !strings.HasSuffix(meta, ";base64") {
			return imageprovider.OutputImage{}, fmt.Errorf("unsupported data URL encoding: expected ;base64")
		}
		mediaType := strings.TrimSuffix(meta, ";base64")
		if mediaType == "" {
			return imageprovider.OutputImage{}, fmt.Errorf("malformed data URL: missing media type")
		}
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return imageprovider.OutputImage{}, fmt.Errorf("decode base64 image data: %w", err)
		}
		return imageprovider.OutputImage{Data: decoded, MediaType: mediaType}, nil
	}
	// Plain URL — validate scheme and return as reference.
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return imageprovider.OutputImage{}, fmt.Errorf("image URL must be an absolute http/https URL: %q", value)
	}
	return imageprovider.OutputImage{URL: value}, nil
}

// normalizeEndpoint parses and validates the endpoint, stripping trailing slashes.
// Only http and https schemes are accepted.
func normalizeEndpoint(endpoint string) (string, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("parse azure foundry endpoint: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("azure foundry endpoint must use http or https scheme")
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("azure foundry endpoint must be an absolute URL")
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimSuffix(parsed.String(), "/"), nil
}
