# AGENTS.md

## Purpose

This repository is `ulduar`, an anonymous multimodal chat app with:

- Go backend
- React + TypeScript frontend
- PostgreSQL for sessions, messages, runs, and attachment metadata
- Azure Blob Storage for uploaded files
- Azure OpenAI Responses API for model execution

Use this file as the default working guide for code agents in this repo.

## Repo Map

- `apps/backend`: Go HTTP API, chat orchestration, persistence, migrations
- `apps/frontend`: React SPA
- `docs/design.md`: v1 product and architecture constraints
- `README.md`: local setup, env vars, and verification commands
- `compose.yaml` / `compose.wsl.yaml`: local full-stack startup

## Product Constraints

Keep these behaviors unless the user explicitly asks to change them:

- The backend is stateless between requests.
- The frontend stores the active `sessionId` in memory only.
- Refreshing or reopening the browser creates a new session.
- Previous sessions remain stored but are not restorable in v1.
- Persist chat state outside process memory.
- v1 supports text, images, and PDFs only.
- Do not add auth, session restore, or chat history browsing as incidental work.
- Azure OpenAI deployment/model names must stay configurable via environment variables.

## Key API Surface

Current main endpoints:

Chat:

- `POST /api/v1/sessions`
- `GET /api/v1/sessions/{sessionId}`
- `POST /api/v1/sessions/{sessionId}/messages`
- `GET /api/v1/sessions/{sessionId}/runs/{runId}/stream`

Image generation (`AZURE_FOUNDRY_ENDPOINT` required for capabilities and create; stream returns 503 for non-terminal generations without a configured provider; read-only retrieval/content endpoints remain available for existing completed generations):

- `GET /api/v1/image-generations/capabilities`
- `POST /api/v1/sessions/{sessionId}/image-generations`
- `GET /api/v1/sessions/{sessionId}/image-generations/{generationId}`
- `GET /api/v1/sessions/{sessionId}/image-generations/{generationId}/stream`
- `GET /api/v1/sessions/{sessionId}/image-generations/{generationId}/assets/{assetId}/content`
- `GET /api/v1/sessions/{sessionId}/image-generations/{generationId}/images/{imageId}/content`

If you change request or response shapes, update backend, frontend, tests, and docs together.

## Working Rules

- Start with `README.md` and `docs/design.md` before making broad changes.
- Prefer small, local edits that preserve the current architecture.
- Keep backend changes compatible with concurrent anonymous sessions.
- Keep attachment handling behind clear abstractions; broader file support is future work.
- When changing behavior, add or update tests close to the affected code.
- Do not silently change env var names, ports, or API routes without updating docs.

## Git Workflow

- The remote `main` branch is protected and requires pull requests for all changes, including admin changes.
- Do not push commits directly to `main`.
- Create a feature branch from `main` for each change, commit there, and open a pull request back into `main`.
- Keep pull requests focused, include relevant verification notes, and update docs alongside behavior or API changes.

## Local Commands

Backend:

```bash
cd apps/backend
go run ./cmd/server
go run ./cmd/migrate up
GOCACHE=/tmp/ulduar-go-build go test ./...
```

Frontend:

```bash
cd apps/frontend
npm install
npm run dev
npm test
npm run build
```

Full stack:

```bash
cp .env.compose.example .env.compose
podman compose --env-file .env.compose up -d --build
```

Env references:

- `.env.example`
- `.env.compose.example`

## Completion Checklist

Before finishing substantial changes:

- Run the most relevant tests for touched areas.
- Run `npm run build` for frontend changes.
- Run `GOCACHE=/tmp/ulduar-go-build go test ./...` for backend changes.
- Update `README.md` or `docs/design.md` when behavior, setup, or architecture expectations change.
