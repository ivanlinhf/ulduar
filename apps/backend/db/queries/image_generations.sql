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
SET provider_job_id = $2,
    status = $3,
    error_code = $4,
    error_message = $5,
    completed_at = $6
WHERE id = $1;
