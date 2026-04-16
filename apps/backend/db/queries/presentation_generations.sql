-- name: CreatePresentationGeneration :one
INSERT INTO presentation_generations (
    session_id,
    prompt,
    provider_name,
    provider_model,
    provider_job_id,
    status,
    error_code,
    error_message,
    completed_at
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8,
    $9
)
RETURNING
    id,
    session_id,
    prompt,
    provider_name,
    provider_model,
    provider_job_id,
    status,
    error_code,
    error_message,
    created_at,
    started_at,
    completed_at;

-- name: GetPresentationGeneration :one
SELECT
    id,
    session_id,
    prompt,
    provider_name,
    provider_model,
    provider_job_id,
    status,
    error_code,
    error_message,
    created_at,
    started_at,
    completed_at
FROM presentation_generations
WHERE id = $1;

-- name: GetPresentationGenerationBySession :one
SELECT
    id,
    session_id,
    prompt,
    provider_name,
    provider_model,
    provider_job_id,
    status,
    error_code,
    error_message,
    created_at,
    started_at,
    completed_at
FROM presentation_generations
WHERE id = $1
  AND session_id = $2;

-- name: ListPresentationGenerationsBySession :many
SELECT
    id,
    session_id,
    prompt,
    provider_name,
    provider_model,
    provider_job_id,
    status,
    error_code,
    error_message,
    created_at,
    started_at,
    completed_at
FROM presentation_generations
WHERE session_id = $1
ORDER BY created_at ASC, id ASC;

-- name: UpdatePresentationGenerationState :execrows
UPDATE presentation_generations
SET provider_name = $2,
    provider_model = $3,
    provider_job_id = $4,
    status = $5,
    error_code = $6,
    error_message = $7,
    completed_at = $8
WHERE id = $1;

-- name: ClaimPendingPresentationGeneration :execrows
UPDATE presentation_generations
SET provider_name = $2,
    provider_model = $3,
    provider_job_id = NULL,
    status = 'running',
    error_code = NULL,
    error_message = NULL,
    started_at = NOW(),
    completed_at = NULL
WHERE id = $1
  AND status = 'pending';

-- name: LockPresentationGenerationForUpdate :one
SELECT
    id,
    session_id,
    prompt,
    provider_name,
    provider_model,
    provider_job_id,
    status,
    error_code,
    error_message,
    created_at,
    started_at,
    completed_at
FROM presentation_generations
WHERE id = $1
FOR UPDATE;
