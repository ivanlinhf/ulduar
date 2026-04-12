package azurefoundry

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ivanlin/ulduar/apps/backend/internal/imageprovider"
)

// ---- NewClient validation ----

func TestNewClientRejectsEmptyEndpoint(t *testing.T) {
	t.Parallel()

	_, err := NewClient("", "key")
	if err == nil {
		t.Fatal("NewClient() error = nil, want error for empty endpoint")
	}
}

func TestNewClientRejectsEmptyAPIKey(t *testing.T) {
	t.Parallel()

	_, err := NewClient("https://foundry.example.com", "")
	if err == nil {
		t.Fatal("NewClient() error = nil, want error for empty api key")
	}
}

func TestNewClientRejectsInvalidEndpointScheme(t *testing.T) {
	t.Parallel()

	_, err := NewClient("ftp://foundry.example.com", "key")
	if err == nil {
		t.Fatal("NewClient() error = nil, want error for non-http/https scheme")
	}
	if !strings.Contains(err.Error(), "http or https") {
		t.Errorf("error = %v", err)
	}
}

func TestNewClientRejectsRelativeEndpoint(t *testing.T) {
	t.Parallel()

	_, err := NewClient("foundry.example.com", "key")
	if err == nil {
		t.Fatal("NewClient() error = nil, want error for relative URL")
	}
}

func TestNewClientAppliesDefaults(t *testing.T) {
	t.Parallel()

	c, err := NewClient("https://foundry.example.com", "key")
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if c.apiVersion != DefaultAPIVersion {
		t.Errorf("apiVersion = %q, want %q", c.apiVersion, DefaultAPIVersion)
	}
	if c.model != DefaultModel {
		t.Errorf("model = %q, want %q", c.model, DefaultModel)
	}
	if c.modelPath != DefaultModelPath {
		t.Errorf("modelPath = %q, want %q", c.modelPath, DefaultModelPath)
	}
}

func TestNewClientAppliesOptions(t *testing.T) {
	t.Parallel()

	c, err := NewClient("https://foundry.example.com/", "key", ClientOptions{
		APIVersion:     "2025-01-01",
		Model:          "FLUX.1-dev",
		ModelPath:      "flux-1-dev",
		RequestTimeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	// trailing slash stripped from endpoint
	if c.endpoint != "https://foundry.example.com" {
		t.Errorf("endpoint = %q", c.endpoint)
	}
	if c.apiVersion != "2025-01-01" {
		t.Errorf("apiVersion = %q", c.apiVersion)
	}
	if c.model != "FLUX.1-dev" {
		t.Errorf("model = %q", c.model)
	}
	if c.modelPath != "flux-1-dev" {
		t.Errorf("modelPath = %q", c.modelPath)
	}
}

func TestNewClientHTTPClientTakesPrecedenceOverRequestTimeout(t *testing.T) {
	t.Parallel()

	custom := &stubHTTPDoer{}
	c, err := NewClient("https://foundry.example.com", "key", ClientOptions{
		RequestTimeout: 30 * time.Second,
		HTTPClient:     custom,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if c.httpClient != custom {
		t.Fatal("httpClient = default client, want explicit HTTPClient to take precedence")
	}
}

// ---- URL construction ----

func TestGenerateURLShape(t *testing.T) {
	t.Parallel()

	c, _ := NewClient("https://foundry.example.com", "key", ClientOptions{
		APIVersion: "preview",
		ModelPath:  "flux-2-pro",
	})

	got := c.generateURL()
	want := "https://foundry.example.com/providers/blackforestlabs/v1/flux-2-pro?api-version=preview"
	if got != want {
		t.Errorf("generateURL() = %q, want %q", got, want)
	}
}

func TestGenerateURLEscapesModelPathAndAPIVersion(t *testing.T) {
	t.Parallel()

	c, _ := NewClient("https://foundry.example.com/base", "key", ClientOptions{
		APIVersion: "preview/2026?",
		ModelPath:  "flux 2/pro",
	})

	got := c.generateURL()
	want := "https://foundry.example.com/base/providers/blackforestlabs/v1/flux%202%2Fpro?api-version=preview%2F2026%3F"
	if got != want {
		t.Errorf("generateURL() = %q, want %q", got, want)
	}
}

func TestJobURLUsesExplicitPollingURL(t *testing.T) {
	t.Parallel()

	c, _ := NewClient("https://foundry.example.com", "key")
	job := imageprovider.ProviderJob{
		JobID:      "job_abc",
		PollingURL: "https://foundry.example.com/result?id=job_abc",
	}
	got, err := c.jobURL(job)
	if err != nil {
		t.Fatalf("jobURL() error = %v", err)
	}
	if got != job.PollingURL {
		t.Errorf("jobURL() = %q, want explicit polling URL %q", got, job.PollingURL)
	}
}

func TestJobURLFallsBackToConstructedURL(t *testing.T) {
	t.Parallel()

	c, _ := NewClient("https://foundry.example.com", "key", ClientOptions{
		APIVersion: "preview",
		ModelPath:  "flux-2-pro",
	})
	job := imageprovider.ProviderJob{JobID: "job_xyz"}
	got, err := c.jobURL(job)
	if err != nil {
		t.Fatalf("jobURL() error = %v", err)
	}
	want := "https://foundry.example.com/providers/blackforestlabs/v1/flux-2-pro/job_xyz?api-version=preview"
	if got != want {
		t.Errorf("jobURL() = %q, want %q", got, want)
	}
}

func TestJobURLRejectsCrossHostPollingURL(t *testing.T) {
	t.Parallel()

	c, _ := NewClient("https://foundry.example.com", "key")
	got, err := c.jobURL(imageprovider.ProviderJob{
		JobID:      "job_xyz",
		PollingURL: "https://poll.example.com/result?id=job_xyz",
	})
	if err != nil {
		t.Fatalf("jobURL() error = %v", err)
	}

	want := "https://foundry.example.com/providers/blackforestlabs/v1/flux-2-pro/job_xyz?api-version=preview"
	if got != want {
		t.Errorf("jobURL() = %q, want %q", got, want)
	}
}

func TestJobURLRequiresJobIDWhenPollingURLIsInvalid(t *testing.T) {
	t.Parallel()

	c, _ := NewClient("https://foundry.example.com", "key")
	_, err := c.jobURL(imageprovider.ProviderJob{
		PollingURL: "https://poll.example.com/result?id=job_xyz",
	})
	if err == nil {
		t.Fatal("jobURL() error = nil, want error when polling URL is invalid and job id is empty")
	}
}

// ---- Auth header ----

func TestGenerateSetsAuthorizationHeader(t *testing.T) {
	t.Parallel()

	var capturedHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeader = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"images":[{"url":"https://img.example.com/out.jpg"}]}`)
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL, "my-secret-key", ClientOptions{HTTPClient: srv.Client()})

	_, err := c.Generate(context.Background(), imageprovider.GenerateRequest{
		Mode:   imageprovider.ModeTextToImage,
		Prompt: "a dog",
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if capturedHeader != "Bearer my-secret-key" {
		t.Errorf("Authorization = %q, want %q", capturedHeader, "Bearer my-secret-key")
	}
}

func TestPollSetsAuthorizationHeader(t *testing.T) {
	t.Parallel()

	var capturedHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeader = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"job_1","status":"InQueue"}`)
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL, "poll-key", ClientOptions{HTTPClient: srv.Client()})

	_, err := c.Poll(context.Background(), imageprovider.ProviderJob{
		JobID:      "job_1",
		PollingURL: srv.URL + "/poll",
	})
	if err != nil {
		t.Fatalf("Poll() error = %v", err)
	}

	if capturedHeader != "Bearer poll-key" {
		t.Errorf("Authorization = %q, want %q", capturedHeader, "Bearer poll-key")
	}
}

// ---- Payload encoding ----

func TestGenerateEncodesTextToImagePayload(t *testing.T) {
	t.Parallel()

	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"images":[]}`)
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL, "key", ClientOptions{Model: "FLUX.2-pro", HTTPClient: srv.Client()})

	_, _ = c.Generate(context.Background(), imageprovider.GenerateRequest{
		Mode:         imageprovider.ModeTextToImage,
		Prompt:       "a sunset",
		Width:        1024,
		Height:       768,
		OutputFormat: "jpeg",
		NumImages:    2,
	})

	var body map[string]any
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("json.Unmarshal(capturedBody) error = %v", err)
	}
	if body["prompt"] != "a sunset" {
		t.Errorf("prompt = %v", body["prompt"])
	}
	if body["width"] != float64(1024) {
		t.Errorf("width = %v", body["width"])
	}
	if body["height"] != float64(768) {
		t.Errorf("height = %v", body["height"])
	}
	if body["output_format"] != "jpeg" {
		t.Errorf("output_format = %v", body["output_format"])
	}
	if body["num_images"] != float64(2) {
		t.Errorf("num_images = %v", body["num_images"])
	}
	if body["model"] != "FLUX.2-pro" {
		t.Errorf("model = %v", body["model"])
	}
}

func TestGenerateEncodesImageEditPayloadWithSingleImage(t *testing.T) {
	t.Parallel()

	imgBytes := []byte("fake-image-data")
	wantEncoded := base64.StdEncoding.EncodeToString(imgBytes)

	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"images":[]}`)
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL, "key", ClientOptions{HTTPClient: srv.Client()})

	_, _ = c.Generate(context.Background(), imageprovider.GenerateRequest{
		Mode:        imageprovider.ModeImageEdit,
		Prompt:      "add a rainbow",
		InputImages: [][]byte{imgBytes},
	})

	var body map[string]any
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("json.Unmarshal(capturedBody) error = %v", err)
	}
	if body["input_image"] != wantEncoded {
		t.Errorf("input_image = %q, want base64 of input image", body["input_image"])
	}
	if _, ok := body["input_image_2"]; ok {
		t.Error("input_image_2 should not be set for a single image")
	}
	if _, ok := body["image_prompt"]; ok {
		t.Error("image_prompt should not be sent to FLUX image edit")
	}
	if _, ok := body["image_prompts"]; ok {
		t.Error("image_prompts should not be sent to FLUX image edit")
	}
}

func TestGenerateEncodesImageEditPayloadWithMultipleImages(t *testing.T) {
	t.Parallel()

	img1 := []byte("img-one")
	img2 := []byte("img-two")

	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"images":[]}`)
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL, "key", ClientOptions{HTTPClient: srv.Client()})

	_, _ = c.Generate(context.Background(), imageprovider.GenerateRequest{
		Mode:        imageprovider.ModeImageEdit,
		Prompt:      "merge images",
		InputImages: [][]byte{img1, img2},
	})

	var body map[string]any
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("json.Unmarshal(capturedBody) error = %v", err)
	}
	if body["input_image"] != base64.StdEncoding.EncodeToString(img1) {
		t.Errorf("input_image = %q", body["input_image"])
	}
	if body["input_image_2"] != base64.StdEncoding.EncodeToString(img2) {
		t.Errorf("input_image_2 = %q", body["input_image_2"])
	}
	if _, ok := body["input_image_3"]; ok {
		t.Error("input_image_3 should not be set when only two images are provided")
	}
	if _, ok := body["image_prompt"]; ok {
		t.Error("image_prompt should not be sent to FLUX image edit")
	}
	if _, ok := body["image_prompts"]; ok {
		t.Error("image_prompts should not be sent to FLUX image edit")
	}
}

func TestGenerateRejectsUnknownMode(t *testing.T) {
	t.Parallel()

	c, _ := NewClient("https://foundry.example.com", "key")

	_, err := c.Generate(context.Background(), imageprovider.GenerateRequest{Mode: "super_res"})
	if err == nil {
		t.Fatal("Generate() error = nil, want error for unsupported mode")
	}
	if !strings.Contains(err.Error(), "unsupported image generation mode") {
		t.Errorf("error = %v", err)
	}
}

// ---- Response normalization ----

func TestGenerateNormalizesImmediateBase64Response(t *testing.T) {
	t.Parallel()

	imgData := []byte("raw-jpeg-bytes")
	encoded := base64.StdEncoding.EncodeToString(imgData)
	dataURL := "data:image/jpeg;base64," + encoded

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := syncResponse{Images: []fluxImage{{URL: dataURL, ContentType: "image/jpeg"}}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL, "key", ClientOptions{HTTPClient: srv.Client()})

	result, err := c.Generate(context.Background(), imageprovider.GenerateRequest{
		Mode:   imageprovider.ModeTextToImage,
		Prompt: "test",
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if !result.Completed() {
		t.Fatal("result.Completed() = false, want true")
	}
	if len(result.Images) != 1 {
		t.Fatalf("len(result.Images) = %d, want 1", len(result.Images))
	}
	img := result.Images[0]
	if string(img.Data) != string(imgData) {
		t.Errorf("img.Data = %q, want %q", img.Data, imgData)
	}
	if img.URL != "" {
		t.Errorf("img.URL = %q, want empty", img.URL)
	}
	if img.MediaType != "image/jpeg" {
		t.Errorf("img.MediaType = %q, want image/jpeg", img.MediaType)
	}
}

func TestGenerateNormalizesImmediateURLResponse(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := syncResponse{Images: []fluxImage{{URL: "https://cdn.example.com/out.jpg"}}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL, "key", ClientOptions{HTTPClient: srv.Client()})

	result, err := c.Generate(context.Background(), imageprovider.GenerateRequest{
		Mode:   imageprovider.ModeTextToImage,
		Prompt: "test",
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if !result.Completed() {
		t.Fatal("result.Completed() = false, want true")
	}
	if len(result.Images) != 1 {
		t.Fatalf("len(result.Images) = %d, want 1", len(result.Images))
	}
	img := result.Images[0]
	if img.URL != "https://cdn.example.com/out.jpg" {
		t.Errorf("img.URL = %q", img.URL)
	}
	if img.Data != nil {
		t.Errorf("img.Data should be nil for URL response")
	}
}

func TestGenerateNormalizesOpenAIStyleBase64Response(t *testing.T) {
	t.Parallel()

	imgData := []byte("raw-png-bytes")
	encoded := base64.StdEncoding.EncodeToString(imgData)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(openAIResponse{
			Data: []openAIImage{{B64JSON: encoded}},
		})
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL, "key", ClientOptions{HTTPClient: srv.Client()})

	result, err := c.Generate(context.Background(), imageprovider.GenerateRequest{
		Mode:   imageprovider.ModeTextToImage,
		Prompt: "test",
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if !result.Completed() {
		t.Fatal("result.Completed() = false, want true")
	}
	if len(result.Images) != 1 {
		t.Fatalf("len(result.Images) = %d, want 1", len(result.Images))
	}
	img := result.Images[0]
	if string(img.Data) != string(imgData) {
		t.Errorf("img.Data = %q, want %q", img.Data, imgData)
	}
	if img.URL != "" {
		t.Errorf("img.URL = %q, want empty", img.URL)
	}
}

func TestGenerateNormalizesOpenAIStyleURLResponse(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(openAIResponse{
			Data: []openAIImage{{URL: "https://cdn.example.com/out.png"}},
		})
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL, "key", ClientOptions{HTTPClient: srv.Client()})

	result, err := c.Generate(context.Background(), imageprovider.GenerateRequest{
		Mode:   imageprovider.ModeTextToImage,
		Prompt: "test",
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if !result.Completed() {
		t.Fatal("result.Completed() = false, want true")
	}
	if len(result.Images) != 1 {
		t.Fatalf("len(result.Images) = %d, want 1", len(result.Images))
	}
	img := result.Images[0]
	if img.URL != "https://cdn.example.com/out.png" {
		t.Errorf("img.URL = %q", img.URL)
	}
	if img.Data != nil {
		t.Errorf("img.Data should be nil for URL response")
	}
}

func TestGenerateNormalizesAsyncJobResponse(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		resp := asyncResponse{
			ID:          "run_abc123",
			Status:      "InQueue",
			FutureLinks: []string{"https://poll.example.com/result?id=run_abc123"},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL, "key", ClientOptions{HTTPClient: srv.Client()})

	result, err := c.Generate(context.Background(), imageprovider.GenerateRequest{
		Mode:   imageprovider.ModeTextToImage,
		Prompt: "async test",
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if result.Completed() {
		t.Fatal("result.Completed() = true, want false for async job")
	}
	if result.Job == nil {
		t.Fatal("result.Job is nil")
	}
	if result.Job.JobID != "run_abc123" {
		t.Errorf("job.JobID = %q, want run_abc123", result.Job.JobID)
	}
	if result.Job.PollingURL != "https://poll.example.com/result?id=run_abc123" {
		t.Errorf("job.PollingURL = %q", result.Job.PollingURL)
	}
}

func TestGenerateRejectsAsyncResponseWithEmptyJobID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		resp := asyncResponse{ID: "", Status: "InQueue"}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL, "key", ClientOptions{HTTPClient: srv.Client()})

	_, err := c.Generate(context.Background(), imageprovider.GenerateRequest{
		Mode:   imageprovider.ModeTextToImage,
		Prompt: "test",
	})
	if err == nil {
		t.Fatal("Generate() error = nil, want error for empty job id")
	}
	if !strings.Contains(err.Error(), "job id") {
		t.Errorf("error = %v", err)
	}
}

func TestGenerateRejectsAsyncResponseWithInvalidPollingURL(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		resp := asyncResponse{
			ID:          "run_1",
			Status:      "InQueue",
			FutureLinks: []string{"javascript:alert(1)"},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL, "key", ClientOptions{HTTPClient: srv.Client()})

	_, err := c.Generate(context.Background(), imageprovider.GenerateRequest{
		Mode:   imageprovider.ModeTextToImage,
		Prompt: "test",
	})
	if err == nil {
		t.Fatal("Generate() error = nil, want error for invalid polling URL")
	}
}

func TestGenerateReturnsAPIErrorOnBadStatus(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":"unauthorized"}`)
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL, "bad-key", ClientOptions{HTTPClient: srv.Client()})

	_, err := c.Generate(context.Background(), imageprovider.GenerateRequest{
		Mode:   imageprovider.ModeTextToImage,
		Prompt: "test",
	})
	if err == nil {
		t.Fatal("Generate() error = nil, want API error")
	}
	var apiErr APIError
	if !isAPIError(err, &apiErr) {
		t.Fatalf("error type = %T, want APIError", err)
	}
	if apiErr.StatusCode != http.StatusUnauthorized {
		t.Errorf("apiErr.StatusCode = %d, want 401", apiErr.StatusCode)
	}
}

// ---- Poll response normalization ----

func TestPollReturnsPendingForInQueueStatus(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"id":"job_1","status":"InQueue"}`)
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL, "key", ClientOptions{HTTPClient: srv.Client()})

	pr, err := c.Poll(context.Background(), imageprovider.ProviderJob{
		JobID:      "job_1",
		PollingURL: srv.URL + "/poll",
	})
	if err != nil {
		t.Fatalf("Poll() error = %v", err)
	}
	if pr.Status != imageprovider.PollStatusPending {
		t.Errorf("pr.Status = %q, want pending", pr.Status)
	}
}

func TestPollReturnsCompletedWithBase64Image(t *testing.T) {
	t.Parallel()

	imgData := []byte("jpeg-content")
	encoded := base64.StdEncoding.EncodeToString(imgData)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		body := pollResponse{
			ID:     "job_1",
			Status: "Ready",
			Result: &pollResult{Sample: "data:image/jpeg;base64," + encoded},
		}
		_ = json.NewEncoder(w).Encode(body)
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL, "key", ClientOptions{HTTPClient: srv.Client()})

	pr, err := c.Poll(context.Background(), imageprovider.ProviderJob{
		JobID:      "job_1",
		PollingURL: srv.URL + "/poll",
	})
	if err != nil {
		t.Fatalf("Poll() error = %v", err)
	}
	if pr.Status != imageprovider.PollStatusCompleted {
		t.Errorf("pr.Status = %q, want completed", pr.Status)
	}
	if len(pr.Images) != 1 {
		t.Fatalf("len(pr.Images) = %d, want 1", len(pr.Images))
	}
	if string(pr.Images[0].Data) != string(imgData) {
		t.Errorf("pr.Images[0].Data = %q, want %q", pr.Images[0].Data, imgData)
	}
}

func TestPollReturnsFailedOnErrorStatus(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		body := pollResponse{ID: "job_1", Status: "Error", Error: "generation failed"}
		_ = json.NewEncoder(w).Encode(body)
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL, "key", ClientOptions{HTTPClient: srv.Client()})

	pr, err := c.Poll(context.Background(), imageprovider.ProviderJob{
		JobID:      "job_1",
		PollingURL: srv.URL + "/poll",
	})
	if err != nil {
		t.Fatalf("Poll() error = %v", err)
	}
	if pr.Status != imageprovider.PollStatusFailed {
		t.Errorf("pr.Status = %q, want failed", pr.Status)
	}
	if pr.Err != "generation failed" {
		t.Errorf("pr.Err = %q, want %q", pr.Err, "generation failed")
	}
}

// ---- decodeImageField ----

func TestDecodeImageFieldHandlesDataURL(t *testing.T) {
	t.Parallel()

	raw := []byte{1, 2, 3, 4}
	encoded := base64.StdEncoding.EncodeToString(raw)
	out, err := decodeImageField("data:image/png;base64," + encoded)
	if err != nil {
		t.Fatalf("decodeImageField() error = %v", err)
	}
	if string(out.Data) != string(raw) {
		t.Errorf("Data = %v, want %v", out.Data, raw)
	}
	if out.MediaType != "image/png" {
		t.Errorf("MediaType = %q, want image/png", out.MediaType)
	}
	if out.URL != "" {
		t.Errorf("URL = %q, want empty", out.URL)
	}
}

func TestDecodeImageFieldHandlesPlainURL(t *testing.T) {
	t.Parallel()

	out, err := decodeImageField("https://cdn.example.com/image.jpg")
	if err != nil {
		t.Fatalf("decodeImageField() error = %v", err)
	}
	if out.URL != "https://cdn.example.com/image.jpg" {
		t.Errorf("URL = %q", out.URL)
	}
	if out.Data != nil {
		t.Errorf("Data should be nil for URL image")
	}
}

func TestDecodeImageFieldRejectsNonBase64DataURL(t *testing.T) {
	t.Parallel()

	_, err := decodeImageField("data:image/png,rawdata")
	if err == nil {
		t.Fatal("decodeImageField() error = nil, want error for non-base64 data URL")
	}
	if !strings.Contains(err.Error(), "base64") {
		t.Errorf("error = %v", err)
	}
}

func TestDecodeImageFieldRejectsUnsafeURL(t *testing.T) {
	t.Parallel()

	for _, unsafe := range []string{"", "javascript:alert(1)", "ftp://example.com/img.jpg"} {
		_, err := decodeImageField(unsafe)
		if err == nil {
			t.Errorf("decodeImageField(%q) error = nil, want error", unsafe)
		}
	}
}

func TestBuildTextToImagePayloadDefaultsNumImagesToOne(t *testing.T) {
	t.Parallel()

	c, _ := NewClient("https://foundry.example.com", "key")
	p := c.buildTextToImagePayload(imageprovider.GenerateRequest{
		Mode:   imageprovider.ModeTextToImage,
		Prompt: "test",
	})
	if p.NumImages != 1 {
		t.Errorf("NumImages = %d, want 1 when not specified", p.NumImages)
	}
}

func TestBuildImageEditPayloadDefaultsNumImagesToOne(t *testing.T) {
	t.Parallel()

	c, _ := NewClient("https://foundry.example.com", "key")
	p := c.buildImageEditPayload(imageprovider.GenerateRequest{
		Mode:   imageprovider.ModeImageEdit,
		Prompt: "test",
	})
	if p.NumImages != 1 {
		t.Errorf("NumImages = %d, want 1 when not specified", p.NumImages)
	}
}

type stubHTTPDoer struct{}

func (d *stubHTTPDoer) Do(req *http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
}

// isAPIError is a helper because errors.As needs a pointer to the target type.
func isAPIError(err error, target *APIError) bool {
	if err == nil {
		return false
	}
	ae, ok := err.(APIError)
	if ok {
		*target = ae
	}
	return ok
}
