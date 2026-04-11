# ulduar

Anonymous multimodal chat app with:

- Go backend
- React frontend
- PostgreSQL for sessions/messages/runs
- Azure Blob Storage for uploaded files
- Azure OpenAI Responses API for chat
- Pluggable image generation provider (Azure AI Foundry FLUX as the initial provider)

## Repo Layout

- [apps/backend](apps/backend)
- [apps/frontend](apps/frontend)
- [docs/design.md](docs/design.md)
- [compose.yaml](compose.yaml)

## Required Environment Variables

Backend app startup validates these:

- `BACKEND_PORT`
  Default is `8080` if unset.
- `BACKEND_READ_HEADER_TIMEOUT`
  Optional duration. Default `5s`.
- `BACKEND_READ_TIMEOUT`
  Optional duration covering request body reads, including uploads. Default `90s`.
- `BACKEND_IDLE_TIMEOUT`
  Optional duration. Default `120s`.
- `BACKEND_SHUTDOWN_TIMEOUT`
  Optional duration. Default `10s`.
- `BACKEND_REQUEST_TIMEOUT`
  Optional duration for non-stream API handlers. Default `15s`.
- `BACKEND_MESSAGE_REQUEST_TIMEOUT`
  Optional duration for `POST /messages`. Default `90s`.
- `DATABASE_URL`
  Example: `postgres://postgres:postgres@localhost:5432/ulduar?sslmode=disable`
- `AZURE_STORAGE_ACCOUNT_NAME`
  Local Azurite value: `devstoreaccount1`
- `AZURE_STORAGE_ACCOUNT_KEY`
  Local Azurite dev key from the env examples
- `AZURE_STORAGE_BLOB_ENDPOINT`
  Manual local example: `http://localhost:10000/devstoreaccount1`
  Compose example: `http://azurite:10000/devstoreaccount1`
- `AZURE_STORAGE_CONTAINER`
  Example: `chat-attachments`
- `AZURE_OPENAI_ENDPOINT`
  Example: `https://your-resource-name.openai.azure.com/`
- `AZURE_OPENAI_API_KEY`
  Azure OpenAI API key
- `AZURE_OPENAI_API_VERSION`
  Current app default is to pass through whatever you set. For the current implementation and examples, use `v1`.
- `AZURE_OPENAI_DEPLOYMENT`
  Azure OpenAI deployment name, for example `gpt-5-chat`
- `AZURE_OPENAI_SYSTEM_PROMPT`
  Optional. If unset, defaults to markdown-friendly response guidance that prefers Markdown when helpful, avoids raw HTML, and still follows user requests for plain text or machine-readable output such as JSON. Set it to an empty string only if you want to disable the default prompt entirely. Set it to a non-empty value to use that exact override.
- `AZURE_OPENAI_ENABLE_WEB_SEARCH`
  Optional boolean. Default `false`. When set to `true`, the backend includes Azure-native `web_search` in Responses API requests, persists final assistant URL citations, and may emit lightweight SSE `tool.status` events. Leave this disabled in production for now; enable it manually only in dev/test.
- `AZURE_OPENAI_REQUEST_TIMEOUT`
  Optional duration for non-stream Responses API calls. Default `90s`.
- `AZURE_OPENAI_STREAM_TIMEOUT`
  Optional duration for streamed Responses API calls. Default `10m`.
- `CHAT_RUN_FINALIZATION_TIMEOUT`
  Optional duration for persisting final run/message state after provider completion or failure. Default `15s`.
- `IMAGE_GENERATION_MAX_REFERENCE_IMAGE_BYTES`
  Optional positive integer byte limit for each image-generation reference upload. Default `20971520` (20 MiB).
- `AZURE_FOUNDRY_ENDPOINT`
  Optional Azure AI Foundry base URL for the FLUX image generation provider. When set, image generation endpoints become active. Requires `AZURE_FOUNDRY_API_KEY`.
- `AZURE_FOUNDRY_API_KEY`
  Optional Azure AI Foundry API key. Required when `AZURE_FOUNDRY_ENDPOINT` is set.
- `AZURE_FOUNDRY_FLUX_API_VERSION`
  Optional. Default `preview`.
- `AZURE_FOUNDRY_FLUX_MODEL`
  Optional. Default `FLUX.2-pro`.
- `AZURE_FOUNDRY_FLUX_MODEL_PATH`
  Optional. Default `flux-2-pro`.
- `AZURE_FOUNDRY_FLUX_REQUEST_TIMEOUT`
  Optional duration for FLUX generation requests. Default `60s`.

Container/entrypoint and frontend build settings:

- `RUN_DB_MIGRATIONS`
  Optional container-only flag. Default `true`. Set it to `false` when database migrations are run separately before backend app rollout.
- `VITE_API_BASE_URL`
  Frontend API base URL. Manual local example: `http://localhost:8080`
- `VITE_APP_VERSION`
  Optional frontend build version identifier used for update detection. Manual local dev defaults to a fresh dev-server version when unset. Container and CI builds should set this explicitly.

Reference files:

- [.env.example](.env.example)
- [.env.compose.example](.env.compose.example)

## Local Startup

### Option 1: Run Services Manually

1. Start PostgreSQL on `localhost:5432`.
2. Start Azurite blob storage on `localhost:10000`.
3. Copy the root env example and adjust hostnames for manual local use.

```bash
cp .env.example .env
```

4. Run database migrations.

```bash
cd apps/backend
DATABASE_URL=postgres://postgres:postgres@localhost:5432/ulduar?sslmode=disable go run ./cmd/migrate up
```

5. Start the backend.

```bash
cd apps/backend
export APP_ENV=development
export BACKEND_PORT=8080
export BACKEND_READ_HEADER_TIMEOUT=5s
export BACKEND_READ_TIMEOUT=90s
export BACKEND_IDLE_TIMEOUT=120s
export BACKEND_SHUTDOWN_TIMEOUT=10s
export BACKEND_REQUEST_TIMEOUT=15s
export BACKEND_MESSAGE_REQUEST_TIMEOUT=90s
export DATABASE_URL=postgres://postgres:postgres@localhost:5432/ulduar?sslmode=disable
export AZURE_STORAGE_ACCOUNT_NAME=devstoreaccount1
export AZURE_STORAGE_ACCOUNT_KEY='Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw=='
export AZURE_STORAGE_BLOB_ENDPOINT=http://localhost:10000/devstoreaccount1
export AZURE_STORAGE_CONTAINER=chat-attachments
export AZURE_OPENAI_ENDPOINT=https://your-resource-name.openai.azure.com/
export AZURE_OPENAI_API_KEY=replace-me
export AZURE_OPENAI_API_VERSION=v1
export AZURE_OPENAI_DEPLOYMENT=gpt-5-chat
# Leave AZURE_OPENAI_SYSTEM_PROMPT unset to use the default markdown-friendly prompt.
# Set it to an empty string only if you want to disable the default prompt explicitly.
# export AZURE_OPENAI_SYSTEM_PROMPT=
# Disabled by default. Enable manually only in dev/test while rolling out Azure-native web_search.
export AZURE_OPENAI_ENABLE_WEB_SEARCH=false
export AZURE_OPENAI_REQUEST_TIMEOUT=90s
export AZURE_OPENAI_STREAM_TIMEOUT=10m
export CHAT_RUN_FINALIZATION_TIMEOUT=15s
export IMAGE_GENERATION_MAX_REFERENCE_IMAGE_BYTES=20971520
# Optional Azure AI Foundry FLUX config for image generation.
# When AZURE_FOUNDRY_ENDPOINT is set, image generation endpoints become active.
# export AZURE_FOUNDRY_ENDPOINT=https://your-foundry-resource.services.ai.azure.com
# export AZURE_FOUNDRY_API_KEY=replace-me
# export AZURE_FOUNDRY_FLUX_API_VERSION=preview
# export AZURE_FOUNDRY_FLUX_MODEL=FLUX.2-pro
# export AZURE_FOUNDRY_FLUX_MODEL_PATH=flux-2-pro
# export AZURE_FOUNDRY_FLUX_REQUEST_TIMEOUT=60s
go run ./cmd/server
```

6. Start the frontend in a second shell.

```bash
cd apps/frontend
export VITE_API_BASE_URL=http://localhost:8080
export VITE_APP_VERSION=local-dev
npm install
npm run dev
```

Frontend will be available at `http://localhost:3000`.

### Optional Azure web search in dev/test

Web search stays disabled by default. To validate it locally, set `AZURE_OPENAI_ENABLE_WEB_SEARCH=true` before starting the backend, then ask a prompt where fresh web results are helpful.

When Azure web search is available and the model chooses to use it:

- the frontend may briefly show `Searching the web...` while the run is in progress
- completed assistant messages may render a `Sources` section from persisted citation URLs
- only final citation metadata is persisted; raw Azure tool logs are not stored

Operational notes:

- the model decides when to search; users do not need a special command
- production rollout is still a separate manual config change
- rollback is simply setting `AZURE_OPENAI_ENABLE_WEB_SEARCH=false` and restarting the backend

### Optional image generation in dev/test

Image generation is disabled by default. To enable it locally, set `AZURE_FOUNDRY_ENDPOINT` and `AZURE_FOUNDRY_API_KEY` before starting the backend.

The image generation layer uses a pluggable provider interface. Azure AI Foundry FLUX (FLUX.2-pro) is the initial configured provider, selected automatically when `AZURE_FOUNDRY_ENDPOINT` and `AZURE_FOUNDRY_API_KEY` are set. Chat still uses Azure OpenAI Responses API; image generation uses the configured image provider independently.

Supported v1 image generation modes:

- `text_to_image` — generate an image from a text prompt only
- `image_edit` — generate a modified image using a prompt and 1–4 reference images (up to `IMAGE_GENERATION_MAX_REFERENCE_IMAGE_BYTES` (default 20 MiB) each)

Each generation always produces exactly 1 output image.

Supported resolutions:

| Key | Width | Height |
|---|---|---|
| `1024x1024` | 1024 | 1024 |
| `1152x896` | 1152 | 896 |
| `896x1152` | 896 | 1152 |
| `1344x768` | 1344 | 768 |
| `768x1344` | 768 | 1344 |
| `1536x1024` | 1536 | 1024 |
| `1024x1536` | 1024 | 1536 |

Image generation endpoints:

Non-session-scoped:

- `GET /api/v1/image-generations/capabilities` — returns available modes, resolutions, and provider info

Session-scoped:

- `POST /api/v1/sessions/{sessionId}/image-generations` — submit a new generation request
- `GET /api/v1/sessions/{sessionId}/image-generations/{generationId}` — poll generation status and asset list
- `GET /api/v1/sessions/{sessionId}/image-generations/{generationId}/stream` — SSE stream for generation progress
- `GET /api/v1/sessions/{sessionId}/image-generations/{generationId}/assets/{assetId}/content` — download a raw generation output asset
- `GET /api/v1/sessions/{sessionId}/image-generations/{generationId}/images/{imageId}/content` — download a generation output image directly

When no provider is configured, `GET /api/v1/image-generations/capabilities` and `POST /api/v1/sessions/{sessionId}/image-generations` return `503 Service Unavailable`, and the stream endpoint returns `503 Service Unavailable` for non-terminal generations. `GET /api/v1/sessions/{sessionId}/image-generations/{generationId}` remains available to fetch an existing generation record regardless of provider configuration, while asset/image download endpoints remain available only when stored outputs exist.

### Option 2: Use `compose.yaml`

The repository includes [compose.yaml](compose.yaml) for:

- `frontend`
- `backend`
- `postgres`
- `azurite`

The compose file wires backend-only service-to-service hostnames internally:

- backend talks to `postgres:5432`
- backend talks to `azurite:10000`

The browser-facing frontend bundle is built with `VITE_API_BASE_URL`, which defaults to `http://localhost:8080` in compose because requests originate from the browser and target the host-published backend port.

If you want an env file for compose-oriented values, start from:

```bash
cp .env.compose.example .env.compose
```

The compose env example includes the browser-side `VITE_API_BASE_URL` because the static frontend image needs that value at build time. It also includes `VITE_APP_VERSION`, which the frontend bakes into the bundle and publishes through `version.json` for reload notifications in already-open tabs. If you change the backend host or port, update `VITE_API_BASE_URL` to match and rebuild the frontend image. If you want to simulate a newer deployed frontend locally, change `VITE_APP_VERSION`, rebuild the frontend image, and then let an already-open tab re-check when it becomes visible, comes back online, or reaches its polling interval. The same `.env.compose` can be used with either [compose.yaml](compose.yaml) or [compose.wsl.yaml](compose.wsl.yaml).

Then start the stack with that env file:

```bash
podman compose --env-file .env.compose up -d --build
```

The backend container runs `migrate up` automatically on startup before serving requests.

### Podman Compose Note

`podman compose` is not a full standalone implementation. It shells out to an installed compose provider such as:

- `podman-compose`
- `docker-compose`

If `podman compose` reports that no compose provider is installed, install one of those providers first and then run:

```bash
podman compose up --build
```

### WSL2 Podman Compose

For Fedora on WSL2, use the WSL-specific compose file instead of the default bridge-network setup:

```bash
cp .env.compose.example .env.compose
podman compose --env-file .env.compose -f compose.wsl.yaml up -d --build
```

This file switches services onto host networking and uses `localhost`-based defaults for database/blob/frontend URLs, which avoids the custom bridge-network failure path seen with rootless Podman on WSL2.

For WSL2, update `.env.compose` with your real Azure OpenAI values. You can leave the database and blob URL values unset if you want the WSL-specific localhost defaults from [compose.wsl.yaml](compose.wsl.yaml) to apply. Keep `VITE_API_BASE_URL` aligned with the backend address that your browser will use.

The backend container runs `migrate up` automatically on startup before serving requests. If you want to disable that behavior for a compose run, set `RUN_DB_MIGRATIONS=false` in `.env.compose`.

## Database Migrations

The backend includes an in-repo migration command under [apps/backend/cmd/migrate](apps/backend/cmd/migrate).

Usage from the backend directory:

```bash
cd apps/backend
DATABASE_URL=postgres://postgres:postgres@localhost:5432/ulduar?sslmode=disable go run ./cmd/migrate
DATABASE_URL=postgres://postgres:postgres@localhost:5432/ulduar?sslmode=disable go run ./cmd/migrate up
DATABASE_URL=postgres://postgres:postgres@localhost:5432/ulduar?sslmode=disable go run ./cmd/migrate down
```

Notes:

- No argument defaults to `up`
- `up` applies all unapplied migrations in order
- `down` rolls back only the latest applied migration
- Migration files live in [apps/backend/db/migrations](apps/backend/db/migrations)

The backend container image also includes the `migrate` binary. Its entrypoint runs `migrate up` automatically only when `RUN_DB_MIGRATIONS` is unset or `true`, so hosted deployments can disable startup migrations and run them once as a separate deploy step instead.

## GitHub Actions Deployments

This repository includes separate Azure Container Apps deploy workflows:

- `.github/workflows/backend-deploy.yml`
- `.github/workflows/frontend-deploy.yml`

Each workflow supports:

- automatic deployment on pushes to `main`, limited to changes under its corresponding app directory
- manual deployment through `workflow_dispatch`

Both workflows check out the triggering commit being deployed (`github.sha`): the pushed `main` commit for automatic runs, or the selected ref's commit for manual `workflow_dispatch` runs. They then authenticate to Azure with GitHub OIDC via `azure/login`, build a new image, push it to Azure Container Registry, and update the target Azure Container App. Each workflow also uses GitHub Actions concurrency control so overlapping deploys for the same app run serially.

Frontend deployment additionally:

- passes `github.sha` into the frontend build as `VITE_APP_VERSION`
- emits a static `version.json` file so older tabs can detect that a newer frontend is deployed
- serves both `index.html` and `version.json` with `Cache-Control: no-store` in the frontend container so update checks do not get stuck on stale shell metadata while hashed JS/CSS assets stay normally cacheable

Backend deployment additionally:

- runs database migrations once per deploy run from the freshly built backend image before the Container App update
- sets `RUN_DB_MIGRATIONS=false` on the backend Container App so backend container startup, restarts, and scaling events do not run migrations

Required GitHub repository secrets:

- `AZURE_CLIENT_ID`
- `AZURE_TENANT_ID`
- `AZURE_SUBSCRIPTION_ID`
- `DATABASE_URL` for backend migration execution

Required GitHub repository variables:

- `AZURE_RESOURCE_GROUP`
- `AZURE_ACR_NAME`
- `BACKEND_CONTAINER_APP_NAME`
- `FRONTEND_CONTAINER_APP_NAME`
- `FRONTEND_VITE_API_BASE_URL`

The workflows assume the Azure resource group, Azure Container Registry, PostgreSQL instance, storage account, backend Container App, and frontend Container App already exist.

## Verification Commands

Backend:

```bash
cd apps/backend
GOCACHE=/tmp/ulduar-go-build go test ./...
```

When `AZURE_OPENAI_ENABLE_WEB_SEARCH=true`, the backend still keeps the existing request and response shapes stable, but assistant message content may include optional citation metadata on text parts and the SSE stream may include `tool.status` events for Azure `web_search` progress. With the flag unset or `false`, backend behavior remains effectively unchanged.

Frontend:

```bash
cd apps/frontend
npm test
npm run build
```
