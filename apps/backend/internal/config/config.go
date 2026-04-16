package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ivanlin/ulduar/apps/backend/internal/azurefoundry"
	"github.com/ivanlin/ulduar/apps/backend/internal/imagegen"
)

const (
	defaultAppEnv                = "development"
	defaultPort                  = "8080"
	defaultReadHeaderTimeout     = 5 * time.Second
	defaultReadTimeout           = 90 * time.Second
	defaultIdleTimeout           = 2 * time.Minute
	defaultShutdownTimeout       = 10 * time.Second
	defaultRequestTimeout        = 15 * time.Second
	defaultMessageRequestTimeout = 90 * time.Second
	defaultOpenAIRequestTimeout  = 90 * time.Second
	defaultOpenAIStreamTimeout   = 10 * time.Minute
	defaultFinalizationTimeout   = 15 * time.Second
	defaultOpenAISystemPrompt    = "Format responses in Markdown when it improves readability. Use plain paragraphs for simple replies. Use lists, tables, and fenced code blocks only when helpful. Do not use raw HTML. If the user asks for plain text or a machine-readable format such as JSON, follow that request instead."
)

type Config struct {
	AppEnv                  string
	Port                    string
	ReadHeaderTimeout       time.Duration
	ReadTimeout             time.Duration
	IdleTimeout             time.Duration
	ShutdownTimeout         time.Duration
	RequestTimeout          time.Duration
	MessageRequestTimeout   time.Duration
	OpenAIRequestTimeout    time.Duration
	OpenAIStreamTimeout     time.Duration
	RunFinalizationTimeout  time.Duration
	DatabaseURL             string
	AzureStorageAccount     string
	AzureStorageKey         string
	AzureStorageBlobURL     string
	AzureStorageContainer   string
	AzureOpenAIEndpoint     string
	AzureOpenAIAPIKey       string
	AzureOpenAIAPIVersion   string
	AzureOpenAIDeployment   string
	AzureOpenAISystemPrompt string
	AzureOpenAIWebSearch    bool
	Presentation            PresentationConfig
	Image                   ImageConfig
}

// PresentationConfig groups all presentation-generation planner settings.
type PresentationConfig struct {
	Endpoint       string
	APIKey         string
	APIVersion     string
	Deployment     string
	SystemPrompt   string
	RequestTimeout time.Duration
	StreamTimeout  time.Duration
}

// ImageConfig groups all image-generation provider settings.
// Each nested struct corresponds to one concrete provider adapter.
type ImageConfig struct {
	MaxReferenceImageBytes int64
	AzureFoundry           FluxConfig
}

// FluxConfig holds settings for the Azure Foundry FLUX adapter.
// Validation only runs when Endpoint is non-empty, so image generation is fully
// optional; callers that do not set AZURE_FOUNDRY_ENDPOINT are unaffected.
type FluxConfig struct {
	Endpoint       string
	APIKey         string
	APIVersion     string
	Model          string
	ModelPath      string
	RequestTimeout time.Duration
}

func Load() (Config, error) {
	webSearchEnabled, err := boolEnvOrDefault("AZURE_OPENAI_ENABLE_WEB_SEARCH", false)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		AppEnv:                  envOrDefault("APP_ENV", defaultAppEnv),
		Port:                    strings.TrimSpace(os.Getenv("BACKEND_PORT")),
		ReadHeaderTimeout:       durationEnvOrDefault("BACKEND_READ_HEADER_TIMEOUT", defaultReadHeaderTimeout),
		ReadTimeout:             durationEnvOrDefault("BACKEND_READ_TIMEOUT", defaultReadTimeout),
		IdleTimeout:             durationEnvOrDefault("BACKEND_IDLE_TIMEOUT", defaultIdleTimeout),
		ShutdownTimeout:         durationEnvOrDefault("BACKEND_SHUTDOWN_TIMEOUT", defaultShutdownTimeout),
		RequestTimeout:          durationEnvOrDefault("BACKEND_REQUEST_TIMEOUT", defaultRequestTimeout),
		MessageRequestTimeout:   durationEnvOrDefault("BACKEND_MESSAGE_REQUEST_TIMEOUT", defaultMessageRequestTimeout),
		OpenAIRequestTimeout:    durationEnvOrDefault("AZURE_OPENAI_REQUEST_TIMEOUT", defaultOpenAIRequestTimeout),
		OpenAIStreamTimeout:     durationEnvOrDefault("AZURE_OPENAI_STREAM_TIMEOUT", defaultOpenAIStreamTimeout),
		RunFinalizationTimeout:  durationEnvOrDefault("CHAT_RUN_FINALIZATION_TIMEOUT", defaultFinalizationTimeout),
		DatabaseURL:             strings.TrimSpace(os.Getenv("DATABASE_URL")),
		AzureStorageAccount:     strings.TrimSpace(os.Getenv("AZURE_STORAGE_ACCOUNT_NAME")),
		AzureStorageKey:         strings.TrimSpace(os.Getenv("AZURE_STORAGE_ACCOUNT_KEY")),
		AzureStorageBlobURL:     strings.TrimSpace(os.Getenv("AZURE_STORAGE_BLOB_ENDPOINT")),
		AzureStorageContainer:   strings.TrimSpace(os.Getenv("AZURE_STORAGE_CONTAINER")),
		AzureOpenAIEndpoint:     strings.TrimSpace(os.Getenv("AZURE_OPENAI_ENDPOINT")),
		AzureOpenAIAPIKey:       strings.TrimSpace(os.Getenv("AZURE_OPENAI_API_KEY")),
		AzureOpenAIAPIVersion:   strings.TrimSpace(os.Getenv("AZURE_OPENAI_API_VERSION")),
		AzureOpenAIDeployment:   strings.TrimSpace(os.Getenv("AZURE_OPENAI_DEPLOYMENT")),
		AzureOpenAISystemPrompt: envOrDefaultUnlessSet("AZURE_OPENAI_SYSTEM_PROMPT", defaultOpenAISystemPrompt),
		AzureOpenAIWebSearch:    webSearchEnabled,
		Presentation: PresentationConfig{
			Endpoint:       strings.TrimSpace(os.Getenv("AZURE_OPENAI_PRESENTATION_ENDPOINT")),
			APIKey:         strings.TrimSpace(os.Getenv("AZURE_OPENAI_PRESENTATION_API_KEY")),
			APIVersion:     strings.TrimSpace(os.Getenv("AZURE_OPENAI_PRESENTATION_API_VERSION")),
			Deployment:     strings.TrimSpace(os.Getenv("AZURE_OPENAI_PRESENTATION_DEPLOYMENT")),
			SystemPrompt:   envOrDefaultUnlessSet("AZURE_OPENAI_PRESENTATION_SYSTEM_PROMPT", defaultOpenAISystemPrompt),
			RequestTimeout: durationEnvOrDefault("AZURE_OPENAI_PRESENTATION_REQUEST_TIMEOUT", defaultOpenAIRequestTimeout),
			StreamTimeout:  durationEnvOrDefault("AZURE_OPENAI_PRESENTATION_STREAM_TIMEOUT", defaultOpenAIStreamTimeout),
		},
		Image: ImageConfig{
			MaxReferenceImageBytes: int64EnvOrDefault("IMAGE_GENERATION_MAX_REFERENCE_IMAGE_BYTES", imagegen.DefaultMaxReferenceImageBytes),
			AzureFoundry: FluxConfig{
				Endpoint:       strings.TrimSpace(os.Getenv("AZURE_FOUNDRY_ENDPOINT")),
				APIKey:         strings.TrimSpace(os.Getenv("AZURE_FOUNDRY_API_KEY")),
				APIVersion:     envOrDefault("AZURE_FOUNDRY_FLUX_API_VERSION", azurefoundry.DefaultAPIVersion),
				Model:          envOrDefault("AZURE_FOUNDRY_FLUX_MODEL", azurefoundry.DefaultModel),
				ModelPath:      envOrDefault("AZURE_FOUNDRY_FLUX_MODEL_PATH", azurefoundry.DefaultModelPath),
				RequestTimeout: durationEnvOrDefault("AZURE_FOUNDRY_FLUX_REQUEST_TIMEOUT", azurefoundry.DefaultTimeout),
			},
		},
	}

	if cfg.Port == "" {
		cfg.Port = defaultPort
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) Validate() error {
	if c.Port == "" {
		return errors.New("port must not be empty")
	}
	port, err := strconv.Atoi(c.Port)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("port must be a valid TCP port: %q", c.Port)
	}

	if err := validatePositiveDuration(c.ReadHeaderTimeout, "backend read header timeout"); err != nil {
		return err
	}
	if err := validatePositiveDuration(c.ReadTimeout, "backend read timeout"); err != nil {
		return err
	}
	if err := validatePositiveDuration(c.IdleTimeout, "backend idle timeout"); err != nil {
		return err
	}
	if err := validatePositiveDuration(c.ShutdownTimeout, "backend shutdown timeout"); err != nil {
		return err
	}
	if err := validatePositiveDuration(c.RequestTimeout, "backend request timeout"); err != nil {
		return err
	}
	if err := validatePositiveDuration(c.MessageRequestTimeout, "backend message request timeout"); err != nil {
		return err
	}
	if err := validatePositiveDuration(c.OpenAIRequestTimeout, "azure openai request timeout"); err != nil {
		return err
	}
	if err := validatePositiveDuration(c.OpenAIStreamTimeout, "azure openai stream timeout"); err != nil {
		return err
	}
	if err := validatePositiveDuration(c.Presentation.RequestTimeout, "azure openai presentation request timeout"); err != nil {
		return err
	}
	if err := validatePositiveDuration(c.Presentation.StreamTimeout, "azure openai presentation stream timeout"); err != nil {
		return err
	}
	if err := validatePositiveDuration(c.RunFinalizationTimeout, "chat run finalization timeout"); err != nil {
		return err
	}
	if err := validatePositiveInt64(c.Image.MaxReferenceImageBytes, "image generation max reference image bytes"); err != nil {
		return err
	}

	if c.DatabaseURL == "" {
		return errors.New("database url must not be empty")
	}
	if err := validateAbsoluteURL(c.DatabaseURL, "database url", "postgres", "postgresql"); err != nil {
		return err
	}

	if c.AzureStorageAccount == "" {
		return errors.New("azure storage account name must not be empty")
	}

	if c.AzureStorageKey == "" {
		return errors.New("azure storage account key must not be empty")
	}

	if c.AzureStorageBlobURL == "" {
		return errors.New("azure storage blob endpoint must not be empty")
	}
	if err := validateAbsoluteURL(c.AzureStorageBlobURL, "azure storage blob endpoint", "http", "https"); err != nil {
		return err
	}

	if c.AzureStorageContainer == "" {
		return errors.New("azure storage container must not be empty")
	}

	if c.AzureOpenAIEndpoint == "" {
		return errors.New("azure openai endpoint must not be empty")
	}
	if err := validateAbsoluteURL(c.AzureOpenAIEndpoint, "azure openai endpoint", "http", "https"); err != nil {
		return err
	}

	if c.AzureOpenAIAPIKey == "" {
		return errors.New("azure openai api key must not be empty")
	}

	if c.AzureOpenAIDeployment == "" {
		return errors.New("azure openai deployment must not be empty")
	}

	if c.Presentation.Endpoint == "" {
		return errors.New("azure openai presentation endpoint must not be empty")
	}
	if err := validateAbsoluteURL(c.Presentation.Endpoint, "azure openai presentation endpoint", "http", "https"); err != nil {
		return err
	}
	if c.Presentation.APIKey == "" {
		return errors.New("azure openai presentation api key must not be empty")
	}
	if c.Presentation.APIVersion == "" {
		return errors.New("azure openai presentation api version must not be empty")
	}
	if c.Presentation.Deployment == "" {
		return errors.New("azure openai presentation deployment must not be empty")
	}

	if c.Image.AzureFoundry.Endpoint != "" {
		if err := validateAbsoluteURL(c.Image.AzureFoundry.Endpoint, "azure foundry endpoint", "http", "https"); err != nil {
			return err
		}
		if c.Image.AzureFoundry.APIKey == "" {
			return errors.New("azure foundry api key must not be empty when endpoint is configured")
		}
		if err := validatePositiveDuration(c.Image.AzureFoundry.RequestTimeout, "azure foundry flux request timeout"); err != nil {
			return err
		}
	}

	return nil
}

func (c Config) HTTPAddress() string {
	return fmt.Sprintf(":%s", c.Port)
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	return value
}

func envOrDefaultUnlessSet(key, fallback string) string {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}

	return value
}

func durationEnvOrDefault(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return -1
	}

	return duration
}

func boolEnvOrDefault(key string, fallback bool) (bool, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean: %w", strings.ToLower(strings.ReplaceAll(key, "_", " ")), err)
	}

	return parsed, nil
}

func int64EnvOrDefault(key string, fallback int64) int64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return -1
	}

	return parsed
}

func validatePositiveDuration(value time.Duration, name string) error {
	if value <= 0 {
		return fmt.Errorf("%s must be a positive duration", name)
	}

	return nil
}

func validatePositiveInt64(value int64, name string) error {
	if value <= 0 {
		return fmt.Errorf("%s must be a positive integer", name)
	}

	return nil
}

func validateAbsoluteURL(raw, name string, allowedSchemes ...string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%s must be a valid URL: %w", name, err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("%s must be an absolute URL", name)
	}

	for _, scheme := range allowedSchemes {
		if parsed.Scheme == scheme {
			return nil
		}
	}

	return fmt.Errorf("%s must use one of: %s", name, strings.Join(allowedSchemes, ", "))
}
