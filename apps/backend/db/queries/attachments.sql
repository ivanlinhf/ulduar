-- name: CreateAttachment :one
INSERT INTO chat_attachments (
    session_id,
    message_id,
    blob_path,
    media_type,
    filename,
    size_bytes,
    sha256,
    provider_file_id
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
    session_id,
    message_id,
    blob_path,
    media_type,
    filename,
    size_bytes,
    sha256,
    provider_file_id,
    created_at;

-- name: ListAttachmentsByMessage :many
SELECT
    id,
    session_id,
    message_id,
    blob_path,
    media_type,
    filename,
    size_bytes,
    sha256,
    provider_file_id,
    created_at
FROM chat_attachments
WHERE message_id = $1
ORDER BY created_at ASC, id ASC;
