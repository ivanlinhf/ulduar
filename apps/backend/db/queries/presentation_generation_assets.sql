-- name: CreatePresentationGenerationAsset :one
INSERT INTO presentation_generation_assets (
    generation_id,
    role,
    sort_order,
    blob_path,
    media_type,
    filename,
    size_bytes,
    sha256
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8
)
RETURNING
    id,
    generation_id,
    role,
    sort_order,
    blob_path,
    media_type,
    filename,
    size_bytes,
    sha256,
    created_at;

-- name: GetPresentationGenerationAsset :one
SELECT
    id,
    generation_id,
    role,
    sort_order,
    blob_path,
    media_type,
    filename,
    size_bytes,
    sha256,
    created_at
FROM presentation_generation_assets
WHERE id = $1;

-- name: GetPresentationGenerationAssetBySession :one
SELECT
    a.id,
    a.generation_id,
    a.role,
    a.sort_order,
    a.blob_path,
    a.media_type,
    a.filename,
    a.size_bytes,
    a.sha256,
    a.created_at
FROM presentation_generation_assets AS a
JOIN presentation_generations AS g
    ON g.id = a.generation_id
WHERE a.id = $1
  AND g.session_id = $2;

-- name: ListPresentationGenerationAssetsByGeneration :many
SELECT
    id,
    generation_id,
    role,
    sort_order,
    blob_path,
    media_type,
    filename,
    size_bytes,
    sha256,
    created_at
FROM presentation_generation_assets
WHERE generation_id = $1
ORDER BY role ASC, sort_order ASC, created_at ASC, id ASC;

-- name: ListPresentationGenerationAssetsByGenerationAndSession :many
SELECT
    a.id,
    a.generation_id,
    a.role,
    a.sort_order,
    a.blob_path,
    a.media_type,
    a.filename,
    a.size_bytes,
    a.sha256,
    a.created_at
FROM presentation_generation_assets AS a
JOIN presentation_generations AS g
    ON g.id = a.generation_id
WHERE a.generation_id = $1
  AND g.session_id = $2
ORDER BY a.role ASC, a.sort_order ASC, a.created_at ASC, a.id ASC;
