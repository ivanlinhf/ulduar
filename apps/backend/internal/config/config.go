package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
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
}

func Load() (Config, error) {
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
		AzureOpenAISystemPrompt: strings.TrimSpace(os.Getenv("AZURE_OPENAI_SYSTEM_PROMPT")),
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
	if err := validatePositiveDuration(c.RunFinalizationTimeout, "chat run finalization timeout"); err != nil {
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

func validatePositiveDuration(value time.Duration, name string) error {
	if value <= 0 {
		return fmt.Errorf("%s must be a positive duration", name)
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
