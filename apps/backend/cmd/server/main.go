package main

import (
	"context"
	"log/slog"
	"net/http"
	"os/signal"
	"strings"
	"syscall"

	"github.com/ivanlin/ulduar/apps/backend/internal/azurefoundry"
	"github.com/ivanlin/ulduar/apps/backend/internal/azureopenai"
	"github.com/ivanlin/ulduar/apps/backend/internal/blobstorage"
	"github.com/ivanlin/ulduar/apps/backend/internal/chat"
	"github.com/ivanlin/ulduar/apps/backend/internal/config"
	"github.com/ivanlin/ulduar/apps/backend/internal/httpapi"
	"github.com/ivanlin/ulduar/apps/backend/internal/imagegen"
	"github.com/ivanlin/ulduar/apps/backend/internal/imageprovider"
	applogging "github.com/ivanlin/ulduar/apps/backend/internal/logging"
	"github.com/ivanlin/ulduar/apps/backend/internal/postgres"
	"github.com/ivanlin/ulduar/apps/backend/internal/presentationgen"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "error", err)
		return
	}

	logger := applogging.NewLogger(cfg.AppEnv)
	slog.SetDefault(logger)

	startupCtx, startupCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer startupCancel()

	dbPool, err := postgres.Connect(startupCtx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("connect postgres", "error", err)
		return
	}
	defer dbPool.Close()

	blobClient, err := blobstorage.Connect(
		cfg.AzureStorageAccount,
		cfg.AzureStorageKey,
		cfg.AzureStorageBlobURL,
		cfg.AzureStorageContainer,
	)
	if err != nil {
		slog.Error("connect blob storage", "error", err)
		return
	}

	openAIClient, err := azureopenai.NewClient(
		cfg.AzureOpenAIEndpoint,
		cfg.AzureOpenAIAPIKey,
		cfg.AzureOpenAIAPIVersion,
		cfg.AzureOpenAIDeployment,
	)
	if err != nil {
		slog.Error("connect azure openai", "error", err)
		return
	}

	service := chat.NewService(dbPool, blobClient, openAIClient, chat.ServiceOptions{
		Instructions:        cfg.AzureOpenAISystemPrompt,
		ResponseTimeout:     cfg.OpenAIRequestTimeout,
		StreamTimeout:       cfg.OpenAIStreamTimeout,
		FinalizationTimeout: cfg.RunFinalizationTimeout,
		EnableWebSearch:     cfg.AzureOpenAIWebSearch,
	})

	var imageProvider imageprovider.ImageProvider
	if strings.TrimSpace(cfg.Image.AzureFoundry.Endpoint) != "" {
		foundryClient, err := azurefoundry.NewClient(
			cfg.Image.AzureFoundry.Endpoint,
			cfg.Image.AzureFoundry.APIKey,
			azurefoundry.ClientOptions{
				APIVersion:     cfg.Image.AzureFoundry.APIVersion,
				Model:          cfg.Image.AzureFoundry.Model,
				ModelPath:      cfg.Image.AzureFoundry.ModelPath,
				RequestTimeout: cfg.Image.AzureFoundry.RequestTimeout,
			},
		)
		if err != nil {
			slog.Error("connect azure foundry", "error", err)
			return
		}
		imageProvider = foundryClient
	}
	imageService := imagegen.NewService(dbPool, blobClient, imagegen.ServiceOptions{
		MaxReferenceImageBytes: cfg.Image.MaxReferenceImageBytes,
		Provider:               imageProvider,
	})
	presentationService := presentationgen.NewService(dbPool, presentationgen.ServiceOptions{
		Planner: presentationgen.PlannerConfig{
			Endpoint:       cfg.Presentation.Endpoint,
			APIKey:         cfg.Presentation.APIKey,
			APIVersion:     cfg.Presentation.APIVersion,
			Deployment:     cfg.Presentation.Deployment,
			SystemPrompt:   cfg.Presentation.SystemPrompt,
			RequestTimeout: cfg.Presentation.RequestTimeout,
			StreamTimeout:  cfg.Presentation.StreamTimeout,
		},
	})

	server := &http.Server{
		Addr: cfg.HTTPAddress(),
		Handler: httpapi.NewHandler(service, httpapi.HandlerOptions{
			RequestTimeout:                        cfg.RequestTimeout,
			MessageRequestTimeout:                 cfg.MessageRequestTimeout,
			ImageGenerationService:                imageService,
			ImageGenerationMaxReferenceImageBytes: cfg.Image.MaxReferenceImageBytes,
		}),
		ErrorLog:          slog.NewLogLogger(logger.With("component", "http_server").Handler(), slog.LevelError),
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}

	slog.Info(
		"starting backend",
		"address", cfg.HTTPAddress(),
		"app_env", cfg.AppEnv,
		"blob_container", blobClient.ContainerName,
		"blob_endpoint", blobClient.Service.URL(),
		"openai_deployment", openAIClient.Deployment(),
		"openai_base_url", openAIClient.BaseURL(),
		"openai_api_version", openAIClient.APIVersion(),
		"presentation_planner_configured", presentationService.PlannerConfigured(),
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		if err != nil && err != http.ErrServerClosed {
			slog.Error("serve http", "error", err)
			return
		}
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("shutdown http server", "error", err)
			return
		}
	}

	slog.Info("closed backend", "app_env", cfg.AppEnv, "address", cfg.HTTPAddress())
}
