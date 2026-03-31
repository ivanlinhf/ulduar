-- name: CreateMessage :one
INSERT INTO chat_messages (
    session_id,
    role,
    content_json,
    status,
    model_name
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5
)
RETURNING id, session_id, role, content_json, status, model_name, created_at;

-- name: GetMessage :one
SELECT id, session_id, role, content_json, status, model_name, created_at
FROM chat_messages
WHERE id = $1;

-- name: ListMessagesBySession :many
SELECT id, session_id, role, content_json, status, model_name, created_at
FROM chat_messages
WHERE session_id = $1
ORDER BY created_at ASC, id ASC;

-- name: UpdateMessageStatusAndModel :exec
UPDATE chat_messages
SET status = $2,
    model_name = $3
WHERE id = $1;

-- name: UpdateMessageContentStatusAndModel :execrows
UPDATE chat_messages
SET content_json = $2,
    status = $3,
    model_name = $4
WHERE id = $1;
