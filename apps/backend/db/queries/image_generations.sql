-- name: CreateImageGeneration :one
INSERT INTO image_generations (
    session_id,
    mode,
    prompt,
    resolution_key,
    width,
    height,
    requested_image_count,
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
    $9,
    $10,
    $11,
    $12,
    $13,
    $14
)
RETURNING
    id,
    session_id,
    mode,
    prompt,
    resolution_key,
    width,
    height,
    requested_image_count,
    provider_name,
    provider_model,
    provider_job_id,
    status,
    error_code,
    error_message,
    created_at,
    completed_at;

-- name: GetImageGeneration :one
SELECT
    id,
    session_id,
    mode,
    prompt,
    resolution_key,
    width,
    height,
    requested_image_count,
    provider_name,
    provider_model,
    provider_job_id,
    status,
    error_code,
    error_message,
    created_at,
    completed_at
FROM image_generations
WHERE id = $1;

-- name: GetImageGenerationBySession :one
SELECT
    id,
    session_id,
    mode,
    prompt,
    resolution_key,
    width,
    height,
    requested_image_count,
    provider_name,
    provider_model,
    provider_job_id,
    status,
    error_code,
    error_message,
    created_at,
    completed_at
FROM image_generations
WHERE id = $1
  AND session_id = $2;

-- name: ListImageGenerationsBySession :many
SELECT
    id,
    session_id,
    mode,
    prompt,
    resolution_key,
    width,
    height,
    requested_image_count,
    provider_name,
    provider_model,
    provider_job_id,
    status,
    error_code,
    error_message,
    created_at,
    completed_at
FROM image_generations
WHERE session_id = $1
ORDER BY created_at ASC, id ASC;

-- name: UpdateImageGenerationState :execrows
UPDATE image_generations
SET provider_name = $2,
    provider_model = $3,
    provider_job_id = $4,
    status = $5,
    error_code = $6,
    error_message = $7,
    completed_at = $8
WHERE id = $1;

-- name: ClaimPendingImageGeneration :execrows
UPDATE image_generations
SET provider_name = $2,
    provider_model = $3,
    provider_job_id = NULL,
    status = 'running',
    error_code = NULL,
    error_message = NULL,
    completed_at = NULL
WHERE id = $1
  AND status = 'pending';
