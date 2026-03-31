-- name: CreateRun :one
INSERT INTO chat_runs (
    session_id,
    user_message_id,
    assistant_message_id,
    provider_response_id,
    status,
    error_code,
    started_at,
    completed_at
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
    user_message_id,
    assistant_message_id,
    provider_response_id,
    status,
    error_code,
    started_at,
    completed_at;

-- name: GetRun :one
SELECT
    id,
    session_id,
    user_message_id,
    assistant_message_id,
    provider_response_id,
    status,
    error_code,
    started_at,
    completed_at
FROM chat_runs
WHERE id = $1;

-- name: UpdateRunState :execrows
UPDATE chat_runs
SET assistant_message_id = $2,
    provider_response_id = $3,
    status = $4,
    error_code = $5,
    completed_at = $6
WHERE id = $1;
