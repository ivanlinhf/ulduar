# Ulduar V1 Design

## Summary

Ulduar v1 is a monorepo web chat application with:

- A stateless Go backend
- A React + TypeScript single-page frontend
- Azure OpenAI as the chat model provider
- A pluggable image generation provider layer (Azure AI Foundry FLUX as the initial provider)
- PostgreSQL for durable chat and image generation persistence
- Azure Blob Storage for attachment and generated image storage

The system must support multiple concurrent anonymous users. Each browser instance creates and holds a `sessionId` only in frontend memory. If the user refreshes or reopens the browser, the SPA creates a new session. Previous sessions remain stored indefinitely but are not discoverable in v1 because there is no authentication, session restore flow, or chat history UI.

As of March 30, 2026, the Azure OpenAI docs checked during planning list `gpt-5-chat` on the Responses API. The implementation should therefore target an Azure deployment of `gpt-5-chat` and keep the deployment/model name configurable through environment variables.

## Goals

- Deliver a working anonymous chat product with text, image, and PDF inputs.
- Persist all chat state so backend restarts do not lose ongoing sessions.
- Stream assistant responses to the frontend.
- Keep the backend stateless so it can serve multiple sessions concurrently.
- Leave room for future features such as broader tool use and agent workflows.

## Non-Goals

- User authentication or authorization
- Chat history browsing or session restore after browser refresh/reopen
- Session deletion or retention policies beyond infinite retention
- Broad file-type support beyond images and PDFs in v1

## Monorepo Structure

Recommended repository layout:

```text
apps/
  backend/
  frontend/
docs/
deploy/            # optional later for cloud manifests
compose.yaml
DEVELOPMENT_PLAN.md
```

No rollout-notes document is required.

## High-Level Architecture

### Frontend

- React + TypeScript SPA
- Creates a new chat session on initial page load
- Stores the active `sessionId` only in a TypeScript runtime variable
- Sends the `sessionId` with every chat request
- Opens an SSE stream to receive incremental assistant output
- When the backend emits Azure web-search progress, shows a transient status and a final `Sources` section for persisted citations

### Backend

- Go HTTP API
- Fully stateless between requests
- Loads all session state from Postgres and Blob Storage as needed
- Calls Azure OpenAI Responses API for chat
- Streams assistant output to the SPA via SSE
- Can optionally attach Azure-native `web_search` behind backend configuration, disabled by default for manual rollout
- Supports a pluggable image generation provider; Azure AI Foundry FLUX is the initial configured adapter, enabled when both `AZURE_FOUNDRY_ENDPOINT` and `AZURE_FOUNDRY_API_KEY` are set

### Storage

- Azure Database for PostgreSQL Flexible Server
  - Stores session, message, run, attachment, and image generation metadata
- Azure Blob Storage
  - Stores raw uploaded files and generated images

### Local Development

- Root `compose.yaml`
- Backend and frontend each built from their own `Dockerfile`
- Compose file must be compatible with both Docker Compose and Podman Compose
- Include local Postgres and Azurite services

## Session Model

### Session lifecycle

- On first SPA load, frontend calls `POST /api/v1/sessions`
- Backend returns a new `sessionId`
- Frontend keeps that `sessionId` in memory only
- Every message request includes the current `sessionId`
- If the browser refreshes or is reopened later, the SPA creates a new session

### Consequences

- Multiple users can use the same backend concurrently
- One browser instance can continue a chat as long as the page remains loaded
- Refreshing the page loses access to the old session from the UI
- Old sessions remain in the database forever

This is acceptable for v1 because usage is expected to be light and there is no requirement to recover or browse prior chats.

## Persistence Model

All chats must be persisted outside process memory.

On each user message:

1. Frontend sends the latest message and any attachments to the backend.
2. Backend stores the user message and attachment metadata.
3. Backend loads all previous messages for the same session.
4. Backend reconstructs the model input from persisted history plus the new turn.
5. Backend sends the request to Azure OpenAI.
6. Backend streams the assistant response back to the frontend.
7. Backend persists the final assistant message and run status.

This flow is correct for v1 and satisfies the reboot-survival requirement.

## Data Model

### `chat_sessions`

- `id`
- `created_at`
- `last_message_at`
- `status`

Suggested `status` values:

- `active`

### `chat_messages`

- `id`
- `session_id`
- `role`
- `content_json`
- `status`
- `model_name`
- `created_at`

Suggested `role` values:

- `system`
- `user`
- `assistant`

Suggested `status` values:

- `pending`
- `completed`
- `failed`

### `chat_attachments`

- `id`
- `session_id`
- `message_id`
- `blob_path`
- `media_type`
- `filename`
- `size_bytes`
- `sha256`
- `provider_file_id` nullable
- `created_at`

### `chat_runs`

- `id`
- `session_id`
- `user_message_id`
- `assistant_message_id`
- `provider_response_id`
- `status`
- `error_code`
- `started_at`
- `completed_at`

Suggested `status` values:

- `pending`
- `streaming`
- `completed`
- `failed`

### Content storage shape

Store message content as normalized JSON rather than plain text so the schema can support:

- Rich multimodal content
- Structured tool call messages
- Tool results
- Future agent-oriented message types

## Attachment Strategy

### V1 supported file types

- Images
- PDFs

### V1 unsupported file types

- `txt`
- `csv`
- `docx`
- `xlsx`
- Any generic binary file not explicitly supported

These should be rejected with a clear validation error in v1.

### Storage behavior

- Store the original uploaded file in Azure Blob Storage
- Store file metadata in Postgres
- For provider submission:
  - Images: send using the supported Azure/OpenAI image input format available at implementation time
  - PDFs: send using the supported Azure/OpenAI PDF/file input path available at implementation time

The attachment preparation logic should sit behind an internal interface so broader file support can be added later without changing the public API.

## Model Integration

### Chat provider

- Azure OpenAI Responses API

### Model

- Default target: configurable deployment of `gpt-5-chat`

### Configuration

Environment variables should cover:

- Azure OpenAI endpoint
- Azure OpenAI API key or credential configuration
- Azure OpenAI API version if needed by the implementation
- Azure model deployment name
- System prompt or default assistant instruction
- Optional Azure-native `web_search` enablement flag, disabled by default and intended for manual dev/test rollout first
- Web-search runs must preserve the existing session model, API shape, and anonymous chat flow while persisting only final citation metadata
- `VITE_IMAGE_GENERATION_ENABLED` (frontend-only build flag): when unset or `false`, the image-generation UI is hidden entirely. Setting this to `true` exposes the image workspace but backend provider configuration is required separately for image generation to function.

### Image generation provider

The backend exposes a provider-neutral `imageprovider.ImageProvider` interface. Concrete adapters implement this interface and keep all provider-specific details isolated from the domain layer.

Azure AI Foundry FLUX (FLUX.2-pro) is the initial configured adapter. It is configured only when both `AZURE_FOUNDRY_ENDPOINT` and `AZURE_FOUNDRY_API_KEY` are set. If `AZURE_FOUNDRY_ENDPOINT` is not set, image generation endpoints return `503 Service Unavailable` and the rest of the backend is unaffected.

Future providers can be added by implementing the interface without changing the HTTP layer.

#### V1 image generation constraints

- Supported modes: `text_to_image` and `image_edit`
- `image_edit` requires 1–4 reference images; `text_to_image` accepts none
- Reference image size limit per upload: 20 MiB (configurable via `IMAGE_GENERATION_MAX_REFERENCE_IMAGE_BYTES`)
- Output count is fixed at 1 image per generation
- Supported resolutions: `1024x1024`, `1152x896`, `896x1152`, `1344x768`, `768x1344`, `1536x1024`, `1024x1536`

### SDK choice

Preferred approach:

- Use a Go SDK only if Azure Responses API support is complete and stable enough

Fallback approach:

- Call Azure OpenAI REST directly from Go

The fallback is important because provider SDK support often lags new model features.

## Backend API

### `POST /api/v1/sessions`

Creates a new session.

Response:

- `sessionId`
- `createdAt`

### `GET /api/v1/sessions/{sessionId}`

Returns session metadata and ordered messages for that session.

This endpoint is mainly useful while the SPA is still open. There is no v1 flow to rediscover a lost `sessionId`.

Completed assistant messages may include optional `inputTokens`, `outputTokens`, and `totalTokens` fields populated from provider-reported usage.

### `POST /api/v1/sessions/{sessionId}/messages`

Accepts:

- User text
- Optional attachments

Behavior:

- Validates the session
- Persists the user message
- Persists attachment metadata
- Creates an assistant placeholder message
- Creates a run record
- Returns a `runId`

### `GET /api/v1/sessions/{sessionId}/runs/{runId}/stream`

Streams assistant output via SSE.

Suggested event types:

- `run.started`
- `message.delta`
- `run.completed`
- `run.failed`

When available, `run.completed` includes optional numeric `inputTokens`, `outputTokens`, and `totalTokens` fields from the provider response usage.

### Image generation endpoints

Provider-dependent endpoints return `503 Service Unavailable` when no image generation provider is configured. This applies to `GET /api/v1/image-generations/capabilities`, `POST /api/v1/sessions/{sessionId}/image-generations`, and the stream endpoint while a generation is still non-terminal. Read-only retrieval endpoints for existing completed generations, including the generation record and stored asset/image content, remain available without an active provider.

#### `GET /api/v1/image-generations/capabilities`

Returns the provider's available modes, supported resolutions, reference image limit, output image count, and provider name.

#### `POST /api/v1/sessions/{sessionId}/image-generations`

Accepts different request formats depending on `mode`:

- `text_to_image`
  - Supports `application/json`; if `Content-Type` is omitted, the backend defaults to JSON
  - JSON fields: `mode`, `prompt`, `resolution`
  - Reference image uploads are forbidden

- `image_edit`
  - Must use `multipart/form-data`; JSON requests are rejected with `400`
  - Multipart fields: `mode`, `prompt`, `resolution`, and one or more image files under `referenceImages` (or `referenceImages[]` for repeated parts)
  - `referenceImages` is required for `image_edit`

Returns `202 Accepted` with a `generationId` and initial `status`.

#### `GET /api/v1/sessions/{sessionId}/image-generations/{generationId}`

Returns the generation record and its asset list (input reference images and output images).

#### `GET /api/v1/sessions/{sessionId}/image-generations/{generationId}/stream`

Streams generation progress via SSE. Events include status transitions and, on completion, the output asset identifiers.

#### `GET /api/v1/sessions/{sessionId}/image-generations/{generationId}/assets/{assetId}/content`

Returns the raw bytes of an output-role generation asset.

#### `GET /api/v1/sessions/{sessionId}/image-generations/{generationId}/images/{imageId}/content`

Returns the raw bytes of an output-role generation image. Served with a long-lived immutable cache header.

### Presentation generation endpoints

Provider-dependent endpoints return `503 Service Unavailable` when no presentation planner is configured. This applies to `GET /api/v1/presentation-generations/capabilities`, `POST /api/v1/sessions/{sessionId}/presentation-generations`, and the stream endpoint while a generation is still non-terminal. Read-only retrieval endpoints for existing completed generations, including the generation record and stored output asset content, remain available without an active planner.

#### `GET /api/v1/presentation-generations/capabilities`

Returns the supported input attachment media types, the output PPTX media type, and the provider name.

#### `POST /api/v1/sessions/{sessionId}/presentation-generations`

Accepts either:

- `application/json` (or missing `Content-Type`, which defaults to JSON)
  - JSON fields: `prompt`
- `multipart/form-data`
  - Multipart fields: `prompt` plus zero or more image/PDF files under `attachments` (or `attachments[]` for repeated parts)

Returns `202 Accepted` with a `generationId` and initial `status`.

#### `GET /api/v1/sessions/{sessionId}/presentation-generations/{generationId}`

Returns the generation record, normalized presentation dialect JSON when available, and its asset list (input attachments and the generated PPTX output).

#### `GET /api/v1/sessions/{sessionId}/presentation-generations/{generationId}/stream`

Streams generation progress via SSE. Events include status transitions and, on completion, the generated PPTX asset identifier.

#### `GET /api/v1/sessions/{sessionId}/presentation-generations/{generationId}/assets/{assetId}/content`

Returns the raw bytes of an output-role generated PPTX asset.

## Frontend Behavior

### Startup

- Create a new session immediately on app load
- Hold `sessionId` in memory only

### Message send

- Submit text and attachments through the message endpoint
- Open an SSE stream using the returned `runId`
- Render assistant output incrementally

### Refresh/reopen

- Start a brand-new session
- Do not attempt to recover the old one

### V1 UI scope

- Chat conversation area
- Composer
- Attachment picker for images and PDFs
- `New` control: always opens a dropdown menu. When image generation is disabled, the menu contains only `New chat`. When image generation is enabled, the menu contains `New chat` and `New image` options.

No history sidebar is needed in v1.

### Image generation UI rollout model

Image generation UI is controlled by two gates:

- **Frontend flag** (`VITE_IMAGE_GENERATION_ENABLED`): when unset or `false`, all image-generation UI is hidden regardless of backend configuration. When `true`, the frontend includes the image-generation entry points and checks backend capabilities to decide whether `New image` is enabled.
- **Backend provider** (`AZURE_FOUNDRY_ENDPOINT` + `AZURE_FOUNDRY_API_KEY`): when unset, the backend returns `503 Service Unavailable` from `GET /api/v1/image-generations/capabilities` and `POST /api/v1/sessions/{sessionId}/image-generations`.

In practice, the image workspace is only reachable when both gates are satisfied: the frontend flag is enabled and backend capabilities report image generation as available. If the frontend flag is enabled but the backend provider is unavailable, the frontend treats image generation as unavailable, `New image` is disabled, and the image workspace cannot be entered from the UI. The chat workspace is unaffected. Setting the backend provider without enabling the frontend flag keeps image generation invisible in the UI.

### Image workspace

When `New image` is selected, the frontend switches to a dedicated image workspace. The image workspace holds its own session, separate from any active chat session. Selecting `New image` again resets the image workspace and starts a fresh image session; the existing chat session is preserved.

The image workspace is a multi-turn view. Each submitted generation becomes a turn displayed in the image timeline. The session persists as long as the page remains loaded; refreshing the browser or switching to a new chat session discards the image workspace state.

### Image generation session model

Each image workspace session is a single backend chat session scoped to image generation. All generations submitted within one image workspace share the same `sessionId`. The session ID is held in memory only, consistent with the chat session model; refreshing the browser loses access to the prior image workspace.

### Prior-image reuse

Users can explicitly reattach a previously uploaded or generated image as a reference for a new generation. Reuse requires a deliberate user action (clicking the reuse button on a prior turn's image); reference images are not carried forward automatically between turns. When the reused image is a previously generated output, the frontend fetches its bytes from the stored content URL and adds it to the current draft's reference image list. When the reused image is one the user previously uploaded, the frontend reuses the in-memory `File` directly without a network fetch.

Uploading a new reference image from disk, reusing a previously uploaded image, and reusing a previously generated output all converge on the same reference image attachment mechanism once the frontend has the file bytes.

## Statelessness and Concurrency

The backend must not assume a single active session. It should support many simultaneous users and sessions, with all request handling based on identifiers in the request itself.

Practical implications:

- No in-process session store
- No sticky-session requirement
- No per-user backend affinity
- All persistence written before or during model execution as needed

## Reliability Expectations

- Persist the user turn before calling the model
- Create the assistant placeholder and run record before streaming starts
- Mark runs/messages failed on provider errors
- Preserve already-completed prior turns even if a run fails
- On backend restart, any session can continue if the client still has the `sessionId`

## Future Extensibility

The implementation should separate the following concerns so future features can be added cleanly:

- Provider adapter
- Conversation assembler
- Attachment preparation pipeline
- Tool execution interface
- Run orchestrator

As implemented in v1, the main extension points to preserve are:

- `chat.ResponseClient`
  Keeps the provider boundary isolated from the HTTP layer so future tool-capable or search-capable providers can be swapped in without changing request handling.
- `chat.BlobStore`
  Keeps attachment persistence behind an interface so future file processing or alternate storage backends can be added without changing chat orchestration.
- Normalized `content_json`
  Message content is already stored as structured JSON parts rather than raw strings, which leaves room for tool calls, tool results, citations, search results, and richer agent state.
- Conversation reconstruction in the chat service
  Provider input is assembled from persisted messages and attachments, which is the right place to inject future tool outputs, retrieval context, or intermediate agent messages.
- Attachment preparation pipeline
  Images and PDFs already go through internal preparation logic before provider submission, which is the seam for adding OCR, text extraction, indexing, or more file types later.
- Run orchestration around `chat_runs`
  The run record already tracks provider IDs, lifecycle state, and failure codes, which can be extended later for multi-step tools, search phases, and agent loops.

Current constraint to keep in mind:

- v1 still assumes a single model-execution step per run. Tool loops, search steps, or agent workflows should extend the run state machine rather than bypassing it.

This should make it easier to add:

- Broader tool use
- Agent loops
- Retrieval or knowledge integrations
- Additional attachment types

## Technology Decisions

### Backend

- Language: Go
- HTTP router: `chi`
- Database access: `pgx` + `sqlc`
- Blob access: Azure SDK for Go

Why Go remains a good fit:

- Mature HTTP and concurrency model
- Strong operational simplicity
- Good fit for stateless backend services
- No critical blocker identified for the required v1 functionality

### Frontend

- Language: TypeScript
- Framework: React
- Build tool: Vite

## Deployment and Local Packaging

### Containers

- `apps/backend/Dockerfile`
- `apps/frontend/Dockerfile`

### Root compose file

Use `compose.yaml` at the repository root with services for:

- `frontend`
- `backend`
- `postgres`
- `azurite`

Compatibility constraints:

- Follow standard Compose spec fields
- Avoid Docker-only extensions
- Use explicit named volumes where needed
- Keep networking simple and portable

## Testing Strategy

### Core behavior

- Session creation works
- Multi-turn chat preserves context
- Backend restart does not break continuation for a still-known `sessionId`

### Concurrency

- Multiple browsers can hold different sessions simultaneously
- No message or stream cross-talk between sessions

### Attachments

- Image upload works end-to-end
- PDF upload works end-to-end
- Unsupported file types are rejected clearly
- File size and MIME validation happen before provider submission

### Streaming

- SSE emits deltas in order
- Completed responses finalize correctly in storage
- Failed runs produce a visible and persisted failure state

### Packaging

- `compose.yaml` boots the stack with Docker Compose
- `compose.yaml` also works with Podman Compose

## Open Constraints To Preserve During Implementation

- Infinite retention
- No auth
- No session recovery after refresh/reopen
- No rollout-notes document
- Design should not paint the project into a corner for future tool or agent features
