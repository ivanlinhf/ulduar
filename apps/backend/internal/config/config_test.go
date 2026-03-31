package config

import (
	"strings"
	"testing"
	"time"
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
}
