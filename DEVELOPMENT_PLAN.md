# Development Plan

This file tracks implementation progress for v1. Update the checkboxes as work completes.

## Phase 0: Project Setup

- [x] Create monorepo directory structure under `apps/`, `docs/`, and optional `deploy/`
- [x] Add root `.gitignore`
- [x] Add root `compose.yaml` compatible with Docker Compose and Podman Compose
- [x] Add backend `Dockerfile`
- [x] Add frontend `Dockerfile`
- [x] Add root environment variable examples for local development

## Phase 1: Backend Scaffold

- [x] Initialize Go module for backend
- [x] Add HTTP server bootstrap and configuration loading
- [x] Add router, middleware, health endpoint, and basic error handling
- [x] Add PostgreSQL connection setup
- [x] Add Azure Blob Storage client setup
- [x] Add Azure OpenAI client abstraction

## Phase 2: Database and Persistence

- [x] Define initial PostgreSQL schema for sessions, messages, attachments, and runs
- [x] Add database migration workflow
- [x] Add typed query layer using `sqlc` or equivalent
- [x] Implement session repository
- [x] Implement message repository
- [x] Implement attachment repository
- [x] Implement run repository

## Phase 3: Attachment Pipeline

- [x] Implement multipart upload handling
- [x] Add validation for supported image types and PDFs
- [x] Add file size limits and MIME checks
- [x] Store raw files in Blob Storage
- [x] Persist attachment metadata in PostgreSQL
- [x] Add provider-input preparation for images
- [x] Add provider-input preparation for PDFs

## Phase 4: Chat Execution Flow

- [x] Implement `POST /api/v1/sessions`
- [x] Implement `GET /api/v1/sessions/{sessionId}`
- [x] Implement `POST /api/v1/sessions/{sessionId}/messages`
- [x] Implement `GET /api/v1/sessions/{sessionId}/runs/{runId}/stream`
- [x] Persist user messages before model execution
- [x] Create assistant placeholder messages and run records
- [x] Reconstruct conversation history from persistence
- [x] Call Azure OpenAI Responses API
- [x] Stream assistant deltas over SSE
- [x] Finalize assistant messages and run status

## Phase 5: Frontend Scaffold

- [x] Initialize React + TypeScript frontend
- [x] Add application shell and chat layout
- [x] Add API client layer
- [x] Create in-memory session bootstrap flow
- [x] Add chat message list and basic message rendering
- [x] Add message composer
- [x] Add attachment picker for images and PDFs
- [x] Add SSE streaming client behavior

## Phase 6: Frontend Chat UX

- [x] Create new session automatically on app load
- [x] Send text-only messages end-to-end
- [x] Send messages with image attachments
- [x] Send messages with PDF attachments
- [x] Render streaming assistant output incrementally
- [x] Show loading, error, and failed-run states
- [x] Optionally add a “New chat” action that creates a fresh session

## Phase 7: Local Dev Experience

- [x] Wire frontend to backend through local environment configuration
- [x] Verify local Postgres persistence across backend restarts
- [x] Verify local Azurite-based attachment storage
- [x] Document local startup steps in `README.md`
- [x] Document required Azure environment variables

## Phase 8: Testing

- [x] Add backend unit tests for repositories and request validation
- [x] Add backend integration tests for session/message/run flows
- [x] Add backend tests for SSE streaming behavior
- [x] Add backend tests for attachment validation
- [x] Add frontend tests for session creation and message flow
- [x] Add frontend tests for streaming state updates
- [x] Manually verify Podman Compose startup

## Phase 9: Hardening

- [x] Add structured logging
- [x] Add request IDs and trace-friendly log fields
- [x] Add configuration validation at startup
- [x] Add sensible upload and request timeouts
- [x] Review API error shapes for frontend compatibility
- [x] Review extension points for future tools, search, and agent workflows

## Completion Criteria

- [ ] Anonymous users can chat with text, images, and PDFs
- [ ] Multiple sessions can be served concurrently by the same stateless backend
- [ ] Chat state survives backend restarts
- [ ] Frontend receives streamed assistant responses
- [ ] Local stack runs through `compose.yaml` or `compose.wsl.yaml`
