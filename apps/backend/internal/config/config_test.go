package config

import (
	"strings"
	"testing"
	"time"

	"github.com/ivanlin/ulduar/apps/backend/internal/azurefoundry"
	"github.com/ivanlin/ulduar/apps/backend/internal/imagegen"
)

func TestLoadAppliesDefaultsAndParsesTimeouts(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("BACKEND_PORT", "8081")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/ulduar?sslmode=disable")
	t.Setenv("AZURE_STORAGE_ACCOUNT_NAME", "devstoreaccount1")
	t.Setenv("AZURE_STORAGE_ACCOUNT_KEY", "secret")
	t.Setenv("AZURE_STORAGE_BLOB_ENDPOINT", "http://localhost:10000/devstoreaccount1")
	t.Setenv("AZURE_STORAGE_CONTAINER", "chat-attachments")
	t.Setenv("AZURE_OPENAI_ENDPOINT", "https://example.openai.azure.com/")
	t.Setenv("AZURE_OPENAI_API_KEY", "secret")
	t.Setenv("AZURE_OPENAI_DEPLOYMENT", "gpt-5-chat")
	t.Setenv("BACKEND_REQUEST_TIMEOUT", "25s")
	t.Setenv("AZURE_OPENAI_STREAM_TIMEOUT", "12m")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Port != "8081" {
		t.Fatalf("cfg.Port = %q", cfg.Port)
	}
	if cfg.RequestTimeout != 25*time.Second {
		t.Fatalf("cfg.RequestTimeout = %v", cfg.RequestTimeout)
	}
	if cfg.OpenAIStreamTimeout != 12*time.Minute {
		t.Fatalf("cfg.OpenAIStreamTimeout = %v", cfg.OpenAIStreamTimeout)
	}
	if cfg.ReadTimeout != defaultReadTimeout {
		t.Fatalf("cfg.ReadTimeout = %v, want %v", cfg.ReadTimeout, defaultReadTimeout)
	}
	if cfg.AzureOpenAISystemPrompt != defaultOpenAISystemPrompt {
		t.Fatalf("cfg.AzureOpenAISystemPrompt = %q", cfg.AzureOpenAISystemPrompt)
	}
	if cfg.AzureOpenAIWebSearch {
		t.Fatal("cfg.AzureOpenAIWebSearch = true, want false by default")
	}
	if cfg.Image.MaxReferenceImageBytes != imagegen.DefaultMaxReferenceImageBytes {
		t.Fatalf("cfg.Image.MaxReferenceImageBytes = %d, want %d", cfg.Image.MaxReferenceImageBytes, imagegen.DefaultMaxReferenceImageBytes)
	}
}

func TestLoadRejectsInvalidValues(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("BACKEND_PORT", "invalid")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/ulduar?sslmode=disable")
	t.Setenv("AZURE_STORAGE_ACCOUNT_NAME", "devstoreaccount1")
	t.Setenv("AZURE_STORAGE_ACCOUNT_KEY", "secret")
	t.Setenv("AZURE_STORAGE_BLOB_ENDPOINT", "http://localhost:10000/devstoreaccount1")
	t.Setenv("AZURE_STORAGE_CONTAINER", "chat-attachments")
	t.Setenv("AZURE_OPENAI_ENDPOINT", "https://example.openai.azure.com/")
	t.Setenv("AZURE_OPENAI_API_KEY", "secret")
	t.Setenv("AZURE_OPENAI_DEPLOYMENT", "gpt-5-chat")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "valid TCP port") {
		t.Fatalf("Load() error = %v", err)
	}

	t.Setenv("BACKEND_PORT", "8080")
	t.Setenv("AZURE_OPENAI_STREAM_TIMEOUT", "bad")

	_, err = Load()
	if err == nil {
		t.Fatal("Load() error = nil, want duration error")
	}
	if !strings.Contains(err.Error(), "azure openai stream timeout") {
		t.Fatalf("Load() error = %v", err)
	}

	t.Setenv("AZURE_OPENAI_STREAM_TIMEOUT", "10m")
	t.Setenv("AZURE_OPENAI_ENABLE_WEB_SEARCH", "maybe")

	_, err = Load()
	if err == nil {
		t.Fatal("Load() error = nil, want boolean error")
	}
	if !strings.Contains(err.Error(), "azure openai enable web search") {
		t.Fatalf("Load() error = %v", err)
	}

	t.Setenv("AZURE_OPENAI_ENABLE_WEB_SEARCH", "false")
	t.Setenv("IMAGE_GENERATION_MAX_REFERENCE_IMAGE_BYTES", "bad")

	_, err = Load()
	if err == nil {
		t.Fatal("Load() error = nil, want integer error")
	}
	if !strings.Contains(err.Error(), "image generation max reference image bytes") {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestLoadUsesCustomSystemPromptOverride(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("BACKEND_PORT", "8080")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/ulduar?sslmode=disable")
	t.Setenv("AZURE_STORAGE_ACCOUNT_NAME", "devstoreaccount1")
	t.Setenv("AZURE_STORAGE_ACCOUNT_KEY", "secret")
	t.Setenv("AZURE_STORAGE_BLOB_ENDPOINT", "http://localhost:10000/devstoreaccount1")
	t.Setenv("AZURE_STORAGE_CONTAINER", "chat-attachments")
	t.Setenv("AZURE_OPENAI_ENDPOINT", "https://example.openai.azure.com/")
	t.Setenv("AZURE_OPENAI_API_KEY", "secret")
	t.Setenv("AZURE_OPENAI_DEPLOYMENT", "gpt-5-chat")
	t.Setenv("AZURE_OPENAI_SYSTEM_PROMPT", "Reply in plain text.")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.AzureOpenAISystemPrompt != "Reply in plain text." {
		t.Fatalf("cfg.AzureOpenAISystemPrompt = %q", cfg.AzureOpenAISystemPrompt)
	}
}

func TestLoadPreservesExplicitlyEmptySystemPrompt(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("BACKEND_PORT", "8080")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/ulduar?sslmode=disable")
	t.Setenv("AZURE_STORAGE_ACCOUNT_NAME", "devstoreaccount1")
	t.Setenv("AZURE_STORAGE_ACCOUNT_KEY", "secret")
	t.Setenv("AZURE_STORAGE_BLOB_ENDPOINT", "http://localhost:10000/devstoreaccount1")
	t.Setenv("AZURE_STORAGE_CONTAINER", "chat-attachments")
	t.Setenv("AZURE_OPENAI_ENDPOINT", "https://example.openai.azure.com/")
	t.Setenv("AZURE_OPENAI_API_KEY", "secret")
	t.Setenv("AZURE_OPENAI_DEPLOYMENT", "gpt-5-chat")
	t.Setenv("AZURE_OPENAI_SYSTEM_PROMPT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.AzureOpenAISystemPrompt != "" {
		t.Fatalf("cfg.AzureOpenAISystemPrompt = %q", cfg.AzureOpenAISystemPrompt)
	}
}

func TestLoadEnablesWebSearchWhenConfigured(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("BACKEND_PORT", "8080")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/ulduar?sslmode=disable")
	t.Setenv("AZURE_STORAGE_ACCOUNT_NAME", "devstoreaccount1")
	t.Setenv("AZURE_STORAGE_ACCOUNT_KEY", "secret")
	t.Setenv("AZURE_STORAGE_BLOB_ENDPOINT", "http://localhost:10000/devstoreaccount1")
	t.Setenv("AZURE_STORAGE_CONTAINER", "chat-attachments")
	t.Setenv("AZURE_OPENAI_ENDPOINT", "https://example.openai.azure.com/")
	t.Setenv("AZURE_OPENAI_API_KEY", "secret")
	t.Setenv("AZURE_OPENAI_DEPLOYMENT", "gpt-5-chat")
	t.Setenv("AZURE_OPENAI_ENABLE_WEB_SEARCH", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !cfg.AzureOpenAIWebSearch {
		t.Fatal("cfg.AzureOpenAIWebSearch = false, want true")
	}
}

func TestLoadUsesConfiguredImageGenerationMaxReferenceImageBytes(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("BACKEND_PORT", "8080")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/ulduar?sslmode=disable")
	t.Setenv("AZURE_STORAGE_ACCOUNT_NAME", "devstoreaccount1")
	t.Setenv("AZURE_STORAGE_ACCOUNT_KEY", "secret")
	t.Setenv("AZURE_STORAGE_BLOB_ENDPOINT", "http://localhost:10000/devstoreaccount1")
	t.Setenv("AZURE_STORAGE_CONTAINER", "chat-attachments")
	t.Setenv("AZURE_OPENAI_ENDPOINT", "https://example.openai.azure.com/")
	t.Setenv("AZURE_OPENAI_API_KEY", "secret")
	t.Setenv("AZURE_OPENAI_DEPLOYMENT", "gpt-5-chat")
	t.Setenv("IMAGE_GENERATION_MAX_REFERENCE_IMAGE_BYTES", "1048576")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Image.MaxReferenceImageBytes != 1048576 {
		t.Fatalf("cfg.Image.MaxReferenceImageBytes = %d, want 1048576", cfg.Image.MaxReferenceImageBytes)
	}
}

func TestLoadAppliesFluxDefaults(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("BACKEND_PORT", "8080")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/ulduar?sslmode=disable")
	t.Setenv("AZURE_STORAGE_ACCOUNT_NAME", "devstoreaccount1")
	t.Setenv("AZURE_STORAGE_ACCOUNT_KEY", "secret")
	t.Setenv("AZURE_STORAGE_BLOB_ENDPOINT", "http://localhost:10000/devstoreaccount1")
	t.Setenv("AZURE_STORAGE_CONTAINER", "chat-attachments")
	t.Setenv("AZURE_OPENAI_ENDPOINT", "https://example.openai.azure.com/")
	t.Setenv("AZURE_OPENAI_API_KEY", "secret")
	t.Setenv("AZURE_OPENAI_DEPLOYMENT", "gpt-5-chat")
	// No AZURE_FOUNDRY_* vars — image provider should be unconfigured with defaults intact.

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	flux := cfg.Image.AzureFoundry
	if flux.Endpoint != "" {
		t.Fatalf("flux.Endpoint = %q, want empty", flux.Endpoint)
	}
	if flux.APIVersion != azurefoundry.DefaultAPIVersion {
		t.Fatalf("flux.APIVersion = %q, want %q", flux.APIVersion, azurefoundry.DefaultAPIVersion)
	}
	if flux.Model != azurefoundry.DefaultModel {
		t.Fatalf("flux.Model = %q, want %q", flux.Model, azurefoundry.DefaultModel)
	}
	if flux.ModelPath != azurefoundry.DefaultModelPath {
		t.Fatalf("flux.ModelPath = %q, want %q", flux.ModelPath, azurefoundry.DefaultModelPath)
	}
	if flux.RequestTimeout != azurefoundry.DefaultTimeout {
		t.Fatalf("flux.RequestTimeout = %v, want %v", flux.RequestTimeout, azurefoundry.DefaultTimeout)
	}
}

func TestLoadValidatesFluxConfigWhenEndpointIsSet(t *testing.T) {
	base := func(tb testing.TB) {
		tb.Helper()
		tb.Setenv("APP_ENV", "development")
		tb.Setenv("BACKEND_PORT", "8080")
		tb.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/ulduar?sslmode=disable")
		tb.Setenv("AZURE_STORAGE_ACCOUNT_NAME", "devstoreaccount1")
		tb.Setenv("AZURE_STORAGE_ACCOUNT_KEY", "secret")
		tb.Setenv("AZURE_STORAGE_BLOB_ENDPOINT", "http://localhost:10000/devstoreaccount1")
		tb.Setenv("AZURE_STORAGE_CONTAINER", "chat-attachments")
		tb.Setenv("AZURE_OPENAI_ENDPOINT", "https://example.openai.azure.com/")
		tb.Setenv("AZURE_OPENAI_API_KEY", "secret")
		tb.Setenv("AZURE_OPENAI_DEPLOYMENT", "gpt-5-chat")
	}

	t.Run("valid", func(t *testing.T) {
		base(t)
		t.Setenv("AZURE_FOUNDRY_ENDPOINT", "https://foundry.example.com")
		t.Setenv("AZURE_FOUNDRY_API_KEY", "foundry-secret")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if cfg.Image.AzureFoundry.Endpoint != "https://foundry.example.com" {
			t.Fatalf("flux.Endpoint = %q", cfg.Image.AzureFoundry.Endpoint)
		}
		if cfg.Image.AzureFoundry.APIKey != "foundry-secret" {
			t.Fatalf("flux.APIKey = %q", cfg.Image.AzureFoundry.APIKey)
		}
	})

	t.Run("missing api key", func(t *testing.T) {
		base(t)
		t.Setenv("AZURE_FOUNDRY_ENDPOINT", "https://foundry.example.com")

		_, err := Load()
		if err == nil {
			t.Fatal("Load() error = nil, want error for missing api key")
		}
		if !strings.Contains(err.Error(), "azure foundry api key") {
			t.Fatalf("Load() error = %v", err)
		}
	})

	t.Run("invalid endpoint url", func(t *testing.T) {
		base(t)
		t.Setenv("AZURE_FOUNDRY_ENDPOINT", "not-a-url")
		t.Setenv("AZURE_FOUNDRY_API_KEY", "key")

		_, err := Load()
		if err == nil {
			t.Fatal("Load() error = nil, want error for invalid endpoint")
		}
		if !strings.Contains(err.Error(), "azure foundry endpoint") {
			t.Fatalf("Load() error = %v", err)
		}
	})

	t.Run("bad request timeout", func(t *testing.T) {
		base(t)
		t.Setenv("AZURE_FOUNDRY_ENDPOINT", "https://foundry.example.com")
		t.Setenv("AZURE_FOUNDRY_API_KEY", "key")
		t.Setenv("AZURE_FOUNDRY_FLUX_REQUEST_TIMEOUT", "notaduration")

		_, err := Load()
		if err == nil {
			t.Fatal("Load() error = nil, want error for bad timeout")
		}
		if !strings.Contains(err.Error(), "azure foundry flux request timeout") {
			t.Fatalf("Load() error = %v", err)
		}
	})
}
